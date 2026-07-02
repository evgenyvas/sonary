// Package ffmpeg
package ffmpeg

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sonary/internal/lib"
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

func (f *FFmpeg) ConvertTrackStream(track *lib.TrackDB, outputStream io.Writer,
	params lib.ConvertParams) error {

	args := []string{"-i", track.Path, "-vn"}

	// for CUE calculate offset
	if track.IsCue {
		startSec := track.CueOffset.Seconds()
		durationSec := track.Duration.Seconds()
		args = append(args, "-ss", fmt.Sprintf("%.3f", startSec), "-t", fmt.Sprintf("%.3f", durationSec))
	}

	args = append(args, "-acodec", "libmp3lame")

	switch params.Mode {
	case "vbr":
		// Variable Bitrate Setup
		quality := "2" // default to medium-high VBR
		if params.Quality != "" {
			quality = params.Quality
		}
		args = append(args, "-q:a", quality)
	case "cbr":
		// Constant Bitrate Setup (Default)
		rate := "192k" // standard default bitrate
		if params.Quality != "" {
			rate = params.Quality + "k"
		}
		args = append(args, "-b:a", rate)
	default:
		return fmt.Errorf("unsupported convert mode: %s", params.Mode)
	}

	// clean original metadata to avoid conflicts
	args = append(args, "-map_metadata", "-1")

	// ID3v2.3 better for audio stream
	args = append(args, "-id3v2_version", "3")

	if track.Title != "" {
		args = append(args, "-metadata", "title="+track.Title)
	}
	if track.Artist != "" {
		args = append(args, "-metadata", "artist="+track.Artist)
	}
	if track.Album != "" {
		args = append(args, "-metadata", "album="+track.Album)
	}
	if track.TrackNumber != 0 {
		args = append(args, "-metadata", "track="+strconv.Itoa(track.TrackNumber))
	}
	if track.Year != 0 {
		args = append(args, "-metadata", "date="+strconv.Itoa(track.Year))
	}
	if track.Genre != "" {
		args = append(args, "-metadata", "genre="+track.Genre)
	}

	// Finalize arguments to output to stdout pipe
	args = append(args, "-f", params.Format, "pipe:1")
	fmt.Printf("%s", args)

	// Initialize and pipe the command
	cmd := exec.Command("ffmpeg", args...)

	// Bind the incoming io.Writer directly to FFmpeg's standard output
	cmd.Stdout = outputStream

	// Optional: Bind Stderr to system console to see FFmpeg logs/errors in terminal
	cmd.Stderr = os.Stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Block and wait until conversion finishes completely
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg execution failed: %w", err)
	}

	return nil
}

func (f *FFmpeg) ConvertTrackToFile(track *lib.TrackDB, tmpOutputPath string,
	params lib.ConvertParams) error {

	args := []string{"-i", track.Path, "-vn"}

	/////////////
	/////////////

	// write the result to disk in the prepared tmpOutputPath
	args = append(args, "-y", tmpOutputPath)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr // execution logs still go to the server console

	return cmd.Run()
}
