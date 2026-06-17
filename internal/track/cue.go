package track

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"sonary/internal/ffmpeg"
	"sonary/internal/lib"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

type CueSheet struct {
	Title     string
	Performer string
	Catalog   string

	REM REMTags

	Files []CueFile
}

type CueFile struct {
	Name        string
	Type        string
	Ext         string
	TotalFrames int

	Tracks []Track
}

type Track struct {
	Number int
	Type   string

	Title     string
	Performer string
	ISRC      string

	REM REMTags

	Indexes []Index

	StartFrame  int
	LengthFrame int
}

type Index struct {
	Number int
	Frame  int
}

type fileInfo struct {
	Name string
	Type string
}

func ParseCue(r io.Reader) (*CueSheet, error) {
	sheet := &CueSheet{
		REM: make(REMTags),
	}

	scanner := bufio.NewScanner(r)

	var currentFile *CueFile
	var currentTrack *Track

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if key, value, ok := parseREM(line); ok {
			if currentTrack != nil {
				if currentTrack.REM == nil {
					currentTrack.REM = make(REMTags)
				}
				currentTrack.REM[key] = value
			} else {
				sheet.REM[key] = value
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "CATALOG "):
			sheet.Catalog =
				strings.TrimSpace(line[len("CATALOG "):])

		case strings.HasPrefix(line, "TITLE "):
			value := unquote(line[len("TITLE "):])
			if currentTrack != nil {
				currentTrack.Title = value
			} else {
				sheet.Title = value
			}

		case strings.HasPrefix(line, "PERFORMER "):
			value := unquote(line[len("PERFORMER "):])
			if currentTrack != nil {
				currentTrack.Performer = value
			} else {
				sheet.Performer = value
			}

		case strings.HasPrefix(line, "FILE "):
			file := parseFileLine(line)
			sheet.Files = append(sheet.Files, CueFile{
				Name: file.Name,
				Type: file.Type,
				Ext:  strings.ToLower(filepath.Ext(file.Name)),
			})
			currentFile = &sheet.Files[len(sheet.Files)-1]
			currentTrack = nil

		case strings.HasPrefix(line, "TRACK "):
			if currentFile == nil {
				continue
			}
			var num int
			var typ string
			fmt.Sscanf(line, "TRACK %d %s", &num, &typ)
			currentFile.Tracks =
				append(currentFile.Tracks, Track{
					Number: num,
					Type:   typ,
				})
			currentTrack =
				&currentFile.Tracks[len(currentFile.Tracks)-1]

		case strings.HasPrefix(line, "ISRC "):
			if currentTrack != nil {
				currentTrack.ISRC =
					strings.TrimSpace(line[len("ISRC "):])
			}

		case strings.HasPrefix(line, "INDEX "):
			if currentTrack == nil {
				continue
			}
			var num int
			var pos string
			fmt.Sscanf(line, "INDEX %d %s", &num, &pos)
			frame, err := parseFrame(pos)
			if err != nil {
				continue
			}
			currentTrack.Indexes =
				append(currentTrack.Indexes, Index{
					Number: num,
					Frame:  frame,
				})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	calculateTrackStarts(sheet)

	return sheet, nil
}

type REMTags map[string]string

func (r REMTags) Get(key string) string {
	return r[key]
}

func (r REMTags) GetInt(key string) (int, error) {
	v := r[key]
	if v == "" {
		return 0, nil
	}

	return strconv.Atoi(v)
}

func (r REMTags) Has(key string) bool {
	_, ok := r[key]
	return ok
}

func (c *CueSheet) Year() int {
	year, err := c.REM.GetInt("DATE")
	if err != nil {
		fmt.Printf("error while parse REM DATE\n")
		return 0
	}
	return year
}

func (c *CueSheet) Genre() string {
	return c.REM.Get("GENRE")
}

func parseREM(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "REM ") {
		return "", "", false
	}
	rest := strings.TrimSpace(line[4:])
	parts := strings.SplitN(rest, " ", 2)

	if len(parts) != 2 {
		return "", "", false
	}

	return parts[0],
		strings.Trim(parts[1], `"`),
		true
}

func parseFileLine(line string) fileInfo {
	first := strings.Index(line, "\"")
	last := strings.LastIndex(line, "\"")
	return fileInfo{
		Name: line[first+1 : last],
		Type: strings.TrimSpace(line[last+1:]),
	}
}

func unquote(s string) string {
	return strings.Trim(s, `"`)
}

func parseFrame(s string) (int, error) {
	var mm, ss, ff int
	_, err :=
		fmt.Sscanf(s, "%d:%d:%d", &mm, &ss, &ff)
	if err != nil {
		return 0, err
	}
	return (mm*60+ss)*75 + ff, nil
}

func calculateTrackStarts(sheet *CueSheet) {
	for fi := range sheet.Files {
		file := &sheet.Files[fi]
		for ti := range file.Tracks {
			track := &file.Tracks[ti]
			for _, idx := range track.Indexes {
				if idx.Number == 1 {
					track.StartFrame = idx.Frame
					break
				}
			}
		}
	}
}

func CalculateDurations(sheet *CueSheet) {
	for fi := range sheet.Files {
		file := &sheet.Files[fi]
		for ti := range file.Tracks {
			current := &file.Tracks[ti]
			if ti < len(file.Tracks)-1 {
				current.LengthFrame =
					file.Tracks[ti+1].StartFrame -
						current.StartFrame
			} else {
				current.LengthFrame =
					file.TotalFrames -
						current.StartFrame
			}
		}
	}
}

func DurationToFrames(d time.Duration) int {
	return int(math.Round(d.Seconds() * 75))
}

func FramesToDuration(frames int) time.Duration {
	return time.Duration(frames) * time.Second / 75
}

func DecodeCue(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}

	encodings := []encoding.Encoding{
		charmap.Windows1251,
		charmap.CodePage866,
		charmap.KOI8R,
	}

	for _, enc := range encodings {
		if decoded, err := enc.NewDecoder().Bytes(data); err == nil {
			return string(decoded)
		}
	}

	return string(data)
}

func scanCue(ff *ffmpeg.FFmpeg, root string, path string, cueFile string) ([]*lib.Track, error) {
	log.Printf("Scanning CUE... '%s'\n", cueFile)
	fullPath := filepath.Join(root, path)
	cueData, err := os.ReadFile(filepath.Join(fullPath, cueFile))
	if err != nil {
		log.Printf("Reading CUE error: %v", err)
		return nil, err
	}

	cue, err := ParseCue(strings.NewReader(DecodeCue(cueData)))
	if err != nil {
		log.Printf("Reading CUE error: %v", err)
		return nil, err
	}
	log.Printf("CUE parsed OK '%s'\n", cueFile)

	// get lyrics if exists
	lyrics, _ := GetLyrics(fullPath)

	for fi := range cue.Files {
		file := &cue.Files[fi]
		duration, err := ff.Duration(filepath.Join(
			fullPath, strings.ReplaceAll(file.Name, "\\", string(os.PathSeparator))))
		if err != nil {
			log.Printf("Load duration error: %v", err)
			return nil, err
		}
		file.TotalFrames = DurationToFrames(duration)
	}

	CalculateDurations(cue)

	album := cue.Title
	if album == "" {
		album = "Unknown Album"
	}
	tracks := []*lib.Track{}
	for fi := range cue.Files {
		file := &cue.Files[fi]
		for ti := range file.Tracks {
			track := &file.Tracks[ti]
			tracks = append(tracks, &lib.Track{
				Path:        filepath.Join(path, file.Name),
				FileType:    strings.ToUpper(strings.ReplaceAll(file.Ext, ".", "")),
				Title:       track.Title,
				Artist:      track.Performer,
				AlbumArtist: cue.Performer,
				Year:        cue.Year(),
				Genre:       cue.Genre(),
				Album:       album,
				TrackNumber: track.Number,
				Duration:    FramesToDuration(track.LengthFrame),
				Lyrics:      GetLyricsForTrack(lyrics, track.Title),
				IsCue:       true,
				CueFile:     filepath.Join(path, cueFile),
				CueOffset:   FramesToDuration(track.StartFrame),
			})
		}
	}

	log.Printf("CUE processed OK '%s'\n", cueFile)
	return tracks, nil
}
