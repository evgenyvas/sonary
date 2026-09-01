package track

import (
	"errors"
	"os"
	"path/filepath"
	db "sonary/internal/database"
	"strings"

	"github.com/dhowden/tag"
)

func scanAudioFile(ad AudioDuration, path string, fileName string) (*db.Track, error) {
	fullPath := filepath.Join(path, fileName)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	duration, err := ad.Duration(fullPath)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	fileTitle := strings.TrimSuffix(fileName, ext)

	meta, err := tag.ReadFrom(f)
	if err != nil {
		if errors.Is(err, tag.ErrNoTagsFound) {
			// tags not found
			return &db.Track{
				Path:     filepath.Join(path, fileName),
				FileType: strings.ToUpper(strings.ReplaceAll(ext, ".", "")),
				Title:    fileTitle,
				Album:    "Unknown Album",
				Duration: duration,
			}, nil
		} else {
			return nil, err
		}
	}

	album := meta.Album()
	if album == "" {
		album = "Unknown Album"
	}

	trackNumber, _ := meta.Track()
	trackTitle := meta.Title()
	if trackTitle == "" {
		trackTitle = fileTitle
	}
	track := &db.Track{
		Path:        filepath.Join(path, fileName),
		FileType:    string(meta.FileType()),
		Title:       trackTitle,
		Artist:      meta.Artist(),
		Album:       album,
		AlbumArtist: meta.AlbumArtist(),
		Year:        meta.Year(),
		Genre:       meta.Genre(),
		TrackNumber: trackNumber,
		Duration:    duration,
		Lyrics:      meta.Lyrics(),
	}

	return track, nil
}
