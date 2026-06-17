// Package ffmpeg
package ffmpeg

import (
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type FFmpeg struct {
	FFmpegPath  string
	FFprobePath string
}

type Metadata struct {
	Duration time.Duration
}

//type FFmpeg interface {
//Duration(path string) (time.Duration, error)
//Convert(src, dst string) error
//Stream(ctx context.Context, src string, w io.Writer) error
//}

func NewFFmpeg() *FFmpeg {
	return &FFmpeg{
		FFmpegPath:  "ffmpeg",
		FFprobePath: "ffprobe",
	}
}

func (f *FFmpeg) Duration(path string) (time.Duration, error) {
	log.Printf("Getting track duration via ffmpeg...'%s'\n", path)

	cmd := exec.Command(
		f.FFprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	out, err := cmd.Output()
	if err != nil {
		log.Printf("FFmpeg error: %v", err)
		return 0, err
	}

	seconds, err := strconv.ParseFloat(
		strings.TrimSpace(string(out)),
		64,
	)
	if err != nil {
		log.Printf("ParseFloat error: %v", err)
		return 0, err
	}

	log.Printf("Track duration determined OK. '%s'\n", path)
	return time.Duration(seconds * float64(time.Second)), nil
}

func (f *FFmpeg) Convert(src string, dst string) error {

	cmd := exec.Command(
		f.FFmpegPath,
		"-i", src,
		"-y",
		dst,
	)

	return cmd.Run()
}
