// Package ffmpeg
package ffmpeg

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sonary/internal/context"
	db "sonary/internal/database"
	"sonary/internal/track"
	"sonary/internal/websocket"
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

func (f *FFmpeg) getInputFormatArgs(path string) []string {
	// Some old FL Studio files have an .mp3 extension and contain valid
	// MP3 data, but FFmpeg 8.x may incorrectly detect them as WAV because
	// of their malformed/legacy header.
	//
	// For .mp3 files, force the MP3 demuxer to handle these files.
	if strings.EqualFold(filepath.Ext(path), ".mp3") {
		return []string{"-f", "mp3", "-i", path}
	}
	return []string{"-i", path}
}

func NewFFmpeg() *FFmpeg {
	return &FFmpeg{
		FFmpegPath:  "ffmpeg",
		FFprobePath: "ffprobe",
	}
}

func (f *FFmpeg) Duration(path string) (time.Duration, error) {
	log.Printf("Getting track duration via ffmpeg...'%s'\n", path)

	args := []string{"-v", "error"}
	args = append(args, f.getInputFormatArgs(path)...)
	args = append(args,
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
	)
	cmd := exec.Command(f.FFprobePath, args...)

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

func (f *FFmpeg) getTrackConvertArgs(t *db.Track, params track.ConvertParams) ([]string, error) {
	args := append(f.getInputFormatArgs(t.Path), "-vn")

	// for CUE calculate offset
	if t.IsCue {
		start := t.CueOffset
		duration := t.Duration

		// add pregap if user wants
		if params.IncludePregap && t.HasPregap {
			start = t.CueOffset - t.PregapDuration
			duration = t.Duration + t.PregapDuration
		}

		args = append(args, "-ss", fmt.Sprintf("%.3f", start.Seconds()), "-t", fmt.Sprintf("%.3f", duration.Seconds()))
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
		return nil, fmt.Errorf("unsupported convert mode: %s", params.Mode)
	}

	// clean original metadata to avoid conflicts
	args = append(args, "-map_metadata", "-1")

	// ID3v2.3 better for audio stream
	args = append(args, "-id3v2_version", "3")

	if t.Title != "" {
		args = append(args, "-metadata", "title="+t.Title)
	}
	if t.Artist != "" {
		args = append(args, "-metadata", "artist="+t.Artist)
	}
	if t.Album != "" {
		args = append(args, "-metadata", "album="+t.Album)
	}
	if t.TrackNumber != 0 {
		args = append(args, "-metadata", "track="+strconv.Itoa(t.TrackNumber))
	}
	if t.Year != 0 {
		args = append(args, "-metadata", "date="+strconv.Itoa(t.Year))
	}
	if t.Genre != "" {
		args = append(args, "-metadata", "genre="+t.Genre)
	}
	return args, nil
}

func (f *FFmpeg) ConvertTrackStream(t *db.Track, params track.ConvertParams,
	outputStream io.Writer) error {

	args, err := f.getTrackConvertArgs(t, params)
	if err != nil {
		return err
	}

	// Finalize arguments to output to stdout pipe
	args = append(args, "-f", params.Format, "pipe:1")

	// Initialize and pipe the command
	cmd := exec.Command(f.FFmpegPath, args...)

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

func sendConvertProgress(userID string, progress int, status string, t *db.Track) {
	hub := websocket.GetHub()
	hub.Send <- websocket.ProgressTrackConvertEvent{
		BaseEvent:  websocket.BaseEvent{UserID: userID},
		Type:       context.EventConvertTrackProgressUpdate,
		Progress:   progress,
		Status:     status,
		TrackID:    t.ID,
		TrackTitle: t.Title,
	}
}

func sendConvertError(userID string, err error, t *db.Track) {
	hub := websocket.GetHub()
	hub.Send <- websocket.MessageEvent{
		BaseEvent: websocket.BaseEvent{UserID: userID},
		Variant:   websocket.MessageVariantError,
		Message:   err.Error(),
	}

	hub.Send <- websocket.ProgressTrackConvertEvent{
		BaseEvent:  websocket.BaseEvent{UserID: userID},
		Type:       context.EventConvertTrackProgressUpdate,
		Status:     websocket.ConvertStatusFailed,
		TrackID:    t.ID,
		TrackTitle: t.Title,
	}
}

func (f *FFmpeg) ConvertFile(t *db.Track,
	params track.ConvertParams,
	targetUserID string) (string, error) {

	st := params.ToString()

	// create temporary file
	tmpFile, err := os.CreateTemp("", "track_"+strconv.Itoa(t.ID)+"_"+st+"_*.mp3")
	if err != nil {
		sendConvertError(targetUserID, err, t)
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	args, err := f.getTrackConvertArgs(t, params)
	if err != nil {
		return "", err
	}

	// ask FFmpeg to output technical progress logging to stderr
	args = append(args, "-progress", "pipe:2", "-y", tmpPath)

	cmd := exec.Command(f.FFmpegPath, args...)

	// intercept Stderr to read logs line by line
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		sendConvertError(targetUserID, err, t)
		return "", fmt.Errorf("failed to get Stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		sendConvertError(targetUserID, err, t)
		return "", fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// track duration
	totalDuration := t.Duration
	// add pregap if user wants
	if params.IncludePregap && t.HasPregap {
		totalDuration = t.Duration + t.PregapDuration
	}

	// read FFmpeg output
	go func() {
		scanner := bufio.NewScanner(stderrPipe)

		for scanner.Scan() {
			line := scanner.Text()

			// FFmpeg sends strings like "out_time_us=45000000" (microseconds)
			if strings.HasPrefix(line, "out_time_us=") {
				timeUs, _ := strconv.ParseFloat(line[12:], 64)
				currentSec := timeUs / 1000000.0

				percent := int((currentSec / totalDuration.Seconds()) * 100)
				if percent > 100 {
					percent = 100
				}
				if percent < 0 {
					percent = 0
				}

				sendConvertProgress(targetUserID, percent, websocket.ConvertStatusProcessing, t)
			}
		}
	}()

	// wait for FFmpeg complete
	if err := cmd.Wait(); err != nil {
		os.Remove(tmpPath)
		sendConvertError(targetUserID, err, t)
		return "", fmt.Errorf("failed to complete ffmpeg: %w", err)
	}

	ct := context.GetConvertContext()
	ct.Cache.Store(strconv.Itoa(t.ID)+"_"+st, tmpPath)

	sendConvertProgress(targetUserID, 100, websocket.ConvertStatusCompleted, t)

	return tmpPath, nil
}
