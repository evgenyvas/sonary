package track

// read lyrics.txt which is nearby

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type TrackLyrics struct {
	Number int
	Title  string
	Lyrics string
}

var trackHeaderRe = regexp.MustCompile(`(?m)^(\d+)\.\s+(.+?)\s*$`)

func ParseLyrics(content string) ([]TrackLyrics, error) {
	matches := trackHeaderRe.FindAllStringSubmatchIndex(content, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("tracks not found")
	}

	var tracks []TrackLyrics

	for i, m := range matches {
		start := m[0]
		end := m[1]

		number := content[m[2]:m[3]]
		title := strings.TrimSpace(content[m[4]:m[5]])

		var lyricsStart int

		// end of header string
		if idx := strings.Index(content[end:], "\n"); idx >= 0 {
			lyricsStart = end + idx + 1
		} else {
			lyricsStart = end
		}

		var lyricsEnd int
		if i+1 < len(matches) {
			lyricsEnd = matches[i+1][0]
		} else {
			lyricsEnd = len(content)
		}

		lyrics := strings.TrimSpace(content[lyricsStart:lyricsEnd])

		var num int
		fmt.Sscanf(number, "%d", &num)

		tracks = append(tracks, TrackLyrics{
			Number: num,
			Title:  title,
			Lyrics: lyrics,
		})

		_ = start
	}

	return tracks, nil
}

func GetLyrics(dirPath string) (map[string]TrackLyrics, error) {
	data, _ := os.ReadFile(filepath.Join(dirPath, "lyrics.txt"))
	if len(data) == 0 { // it's OK if no lyrics
		return nil, nil
	}

	tracks, err := ParseLyrics(string(data))
	if err != nil {
		return nil, err
	}

	lyricsByTrack := make(map[string]TrackLyrics)
	for _, track := range tracks {
		lyricsByTrack[track.Title] = track
	}

	return lyricsByTrack, nil
}

func GetLyricsForTrack(lyrics map[string]TrackLyrics, trackTitle string) string {
	var res string
	if tl, ok := lyrics[trackTitle]; ok {
		res = tl.Lyrics
	}
	return res
}
