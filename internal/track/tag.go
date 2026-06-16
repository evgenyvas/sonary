package track

import (
	"os"
	"path/filepath"
	"sonary/internal/ffmpeg"
	"sonary/internal/lib"

	"github.com/dhowden/tag"
)

func scanAudioFile(ff *ffmpeg.FFmpeg, root string, path string, fileName string) (*lib.Track, error) {
	fullPath := filepath.Join(root, path, fileName)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	meta, err := tag.ReadFrom(f)
	if err != nil {
		return nil, err
	}

	duration, err := ff.Duration(fullPath)
	if err != nil {
		return nil, err
	}

	trackNumber, _ := meta.Track()
	track := &lib.Track{
		Path:        filepath.Join(path, fileName),
		FileType:    string(meta.FileType()),
		Title:       meta.Title(),
		Artist:      meta.Artist(),
		Album:       meta.Album(),
		AlbumArtist: meta.AlbumArtist(),
		Year:        meta.Year(),
		Genre:       meta.Genre(),
		TrackNumber: trackNumber,
		Duration:    duration,
		Lyrics:      meta.Lyrics(),
	}

	return track, nil
}
