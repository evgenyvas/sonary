package track

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	db "sonary/internal/database"
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

	Tracks []CueTrack
}

type CueTrack struct {
	Number int
	Type   string

	Title     string
	Performer string
	ISRC      string

	REM REMTags

	Indexes []CueIndex

	StartFrame       int // INDEX 01
	EndBoundaryFrame int // The point where this track is guaranteed to end
	LengthFrame      int
	HasPregap        bool
}

type CueIndex struct {
	Number int
	Frame  int
}

type CueFileInfo struct {
	Name string
	Type string
}

func ParseCue(r io.Reader) (*CueSheet, error) {
	sheet := &CueSheet{
		REM: make(REMTags),
	}

	scanner := bufio.NewScanner(r)

	var currentFile *CueFile
	var currentTrack *CueTrack

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
			if file == nil {
				continue
			}
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
			if n, err := fmt.Sscanf(line, "TRACK %d %s", &num, &typ); err != nil || n < 2 {
				continue
			}
			currentFile.Tracks =
				append(currentFile.Tracks, CueTrack{
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
			if num == 0 {
				currentTrack.HasPregap = true
			}
			frame, err := parseFrame(pos)
			if err != nil {
				continue
			}
			currentTrack.Indexes =
				append(currentTrack.Indexes, CueIndex{
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

func parseFileLine(line string) *CueFileInfo {
	line = strings.TrimSpace(line[5:]) // Remove prefix "FILE "

	// Variant 1: filename inside quotes
	if strings.HasPrefix(line, "\"") {
		last := strings.LastIndex(line, "\"")
		if last > 0 {
			return &CueFileInfo{
				Name: line[1:last],
				Type: strings.TrimSpace(line[last+1:]),
			}
		}
	}

	// Variant 2: filename without quotes (split by last space)
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		return &CueFileInfo{
			Name: strings.Join(parts[:len(parts)-1], " "),
			Type: parts[len(parts)-1],
		}
	}
	return nil
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

			// we need two different points
			minFrame := math.MaxInt32

			for _, idx := range track.Indexes {
				// INDEX 00 or 01 - take the very first one to determine the physical boundary
				if idx.Frame < minFrame {
					minFrame = idx.Frame
				}
				// INDEX 01 - this is the official beginning of music
				if idx.Number == 1 {
					track.StartFrame = idx.Frame
				}
			}
			track.EndBoundaryFrame = minFrame
		}
	}
}

func CalculateDurations(sheet *CueSheet) {
	for fi := range sheet.Files {
		file := &sheet.Files[fi]
		for ti := range file.Tracks {
			current := &file.Tracks[ti]
			if ti < len(file.Tracks)-1 {
				// count the duration strictly BEFORE THE START (INDEX 00/01) of the next track
				nextTrack := &file.Tracks[ti+1]
				current.LengthFrame = nextTrack.EndBoundaryFrame - current.StartFrame
			} else {
				// for the last track, we count to the end of the entire file.
				// calculate the pregap length of the current (last) track
				pregapLength := current.StartFrame - current.EndBoundaryFrame
				// subtract the pregap from the total file length
				current.LengthFrame = file.TotalFrames - current.StartFrame - pregapLength
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
		charmap.Windows1252,
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

func scanCue(ad AudioDuration, path string, cueFile string) ([]*db.Track, error) {
	log.Printf("Scanning CUE... '%s'\n", cueFile)
	cueData, err := os.ReadFile(filepath.Join(path, cueFile))
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
	lyrics, _ := GetLyrics(path)

	for fi := range cue.Files {
		file := &cue.Files[fi]
		duration, err := ad.Duration(filepath.Join(
			path, strings.ReplaceAll(file.Name, "\\", string(os.PathSeparator))))
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
	// create track objects for insert into database
	tracks := []*db.Track{}
	for fi := range cue.Files {
		file := &cue.Files[fi]
		for ti := range file.Tracks {
			track := &file.Tracks[ti]
			var pregapDuration time.Duration
			if track.HasPregap {
				pregapDuration = FramesToDuration(track.StartFrame - track.EndBoundaryFrame)
			}
			tracks = append(tracks, &db.Track{
				Path:           filepath.Join(path, strings.ReplaceAll(file.Name, "\\", string(os.PathSeparator))),
				FileType:       strings.ToUpper(strings.ReplaceAll(file.Ext, ".", "")),
				Title:          track.Title,
				Artist:         track.Performer,
				AlbumArtist:    cue.Performer,
				Year:           cue.Year(),
				Genre:          cue.Genre(),
				Album:          album,
				TrackNumber:    track.Number,
				Duration:       FramesToDuration(track.LengthFrame),
				HasPregap:      track.HasPregap,
				PregapDuration: pregapDuration,
				Lyrics:         GetLyricsForTrack(lyrics, track.Title),
				IsCue:          true,
				CueFile:        filepath.Join(path, cueFile),
				CueOffset:      FramesToDuration(track.StartFrame),
			})
		}
	}

	log.Printf("CUE processed OK '%s'\n", cueFile)
	return tracks, nil
}
