package track

import (
	"errors"
	"os"
	"path/filepath"
	"sonary/internal/ffmpeg"
	"sonary/internal/lib"
	"strings"

	"github.com/dhowden/tag"
)

func scanAudioFile(ff *ffmpeg.FFmpeg, root string, path string, fileName string) (*lib.Track, error) {
	fullPath := filepath.Join(root, path, fileName)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	duration, err := ff.Duration(fullPath)
	if err != nil {
		return nil, err
	}

	meta, err := tag.ReadFrom(f)
	if err != nil {
		if errors.Is(err, tag.ErrNoTagsFound) {
			// tags not found
			ext := strings.ToLower(filepath.Ext(fileName))
			return &lib.Track{
				Path:     filepath.Join(path, fileName),
				FileType: strings.ToUpper(strings.ReplaceAll(ext, ".", "")),
				Title:    strings.TrimSuffix(fileName, ext),
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
	track := &lib.Track{
		Path:        filepath.Join(path, fileName),
		FileType:    string(meta.FileType()),
		Title:       meta.Title(),
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
