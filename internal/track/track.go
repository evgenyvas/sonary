// Package track
package track

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sonary/internal/config"
	"sonary/internal/database"
	"sonary/internal/ffmpeg"
	"sonary/internal/lib"
	"strings"
	"time"
)

func SyncDirectories() (map[string]any, error) {
	writeDB := database.Writer()
	cfg := config.GetConfig()

	// At first - search for music dirs and sync them with database
	var musicDirs = map[string]lib.DirScan{}
	for _, root := range cfg.RootPaths {
		log.Printf("Starting to read root directory '%s'\n", root)
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))

			switch ext {
			case ".cue", ".mp3", ".flac", ".ogg", ".m4a", ".ape", ".wav":
				dirPath := filepath.Dir(path)
				if dir, ok := musicDirs[dirPath]; ok {
					// mtime max for files inside
					fileInfo, err := os.Stat(path)
					if err != nil {
						log.Printf("Loading file state error: %v", err)
						return err
					}
					fileMtime := fileInfo.ModTime().Unix()
					if fileMtime > dir.Mtime {
						musicDirs[dirPath] = lib.DirScan{
							Mtime:    fileMtime,
							LastScan: 0,
						}
					}
				} else {
					dirInfo, err := os.Stat(dirPath)
					if err != nil {
						log.Printf("Loading directory state error: %v", err)
						return err
					}
					// Skip the root directory itself (which evaluates to ".")
					if dirPath == "." {
						return nil
					}
					musicDirs[dirPath] = lib.DirScan{
						Mtime:    dirInfo.ModTime().Unix(),
						LastScan: 0,
					}
				}
			}

			return nil
		})
	}

	dirExists, err := database.GetDirectories(writeDB)
	if err != nil {
		log.Printf("Loading directories error: %v", err)
		return nil, err
	}
	log.Println("Directories list loaded OK")

	var dirsUpdate = map[string]lib.DirDB{}
	// compare dirs with dirs from db
	for path, dirDB := range dirExists {
		if dir, ok := musicDirs[path]; ok {
			if dirDB.Mtime != dir.Mtime {
				dirDB.Mtime = dir.Mtime
				dirsUpdate[path] = dirDB
			}
			delete(musicDirs, path)
			delete(dirExists, path)
		}
	}

	// new dirs
	if len(musicDirs) > 0 {
		log.Printf("to add: %d\n", len(musicDirs))
		if err := database.SaveDirectories(writeDB, musicDirs); err != nil {
			return nil, err
		}
	}

	// modified dirs - update mtime
	if len(dirsUpdate) > 0 {
		log.Printf("to update: %d\n", len(dirsUpdate))
		if err := database.UpdateDirectoriesMtime(writeDB, dirsUpdate); err != nil {
			return nil, err
		}
	}

	// dirs to delete
	if len(dirExists) > 0 {
		log.Printf("to delete: %d\n", len(dirExists))
		if err := database.DeleteDirectories(writeDB, dirExists); err != nil {
			return nil, err
		}
	}

	log.Println("Directory sync complete.")

	return map[string]any{
		"num":    len(musicDirs) + len(dirsUpdate),
		"add":    len(musicDirs),
		"update": len(dirsUpdate),
		"delete": len(dirExists),
	}, nil
}

func FormatTrackDuration(duration time.Duration) string {
	minutes := int(duration / time.Minute)
	seconds := int((duration % time.Minute) / time.Second)
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

type DirectoryScanner struct {
	Path           string
	Dir            *lib.DirDB
	DB             *sql.DB
	Entries        []os.DirEntry
	FF             *ffmpeg.FFmpeg
	SkipFiles      map[string]struct{}
	Images         []lib.DirectoryImage
	ImageOrder     int
	Thumbnails     ThumbnailGenerator
	MainFrontFound bool
}

func NewDirectoryScanner(path string) (*DirectoryScanner, error) {
	dir, err := database.GetDirectory(database.Reader(), path)
	if err != nil {
		log.Printf("Get directory data error: %v", path)
		return nil, err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		log.Printf("Loading directory content error: %v", path)
		return nil, err
	}

	cfg := config.GetConfig()

	return &DirectoryScanner{
		Path:    path,
		Dir:     dir,
		DB:      database.Writer(),
		Entries: entries,
		FF:      ffmpeg.NewFFmpeg(),
		Thumbnails: ThumbnailGenerator{
			CacheDir: cfg.CacheDir,
		},
	}, nil
}

func (s *DirectoryScanner) ShouldScan() bool {
	if s.Dir.LastScan != 0 {
		log.Printf("Directory skipping. '%s'\n", s.Path)
		return false
	}
	return true
}

func (s *DirectoryScanner) processCueFiles() error {
	var tracks []*lib.Track
	s.SkipFiles = make(map[string]struct{})
	for _, entry := range s.Entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".cue" {
			continue
		}

		// skip replay gain .cue
		if slices.ContainsFunc([]string{" [RG].cue", "RG.cue"}, func(s string) bool {
			return strings.HasSuffix(filepath.Base(entry.Name()), s)
		}) {
			log.Printf("Skipped %v", entry.Name())
			continue
		}

		// parse CUE
		cueTracks, err := scanCue(s.FF, s.Dir.Path, entry.Name())
		if err != nil {
			log.Printf("Scan CUE error: %v", s.Path)
			return err
		}
		log.Printf("CUE scanned successfully '%s'\n", s.Path)

		s.SkipFiles[entry.Name()] = struct{}{}

		for _, track := range cueTracks {
			relTrackPath, err := filepath.Rel(s.Dir.Path, track.Path)
			if err != nil {
				return err
			}
			s.SkipFiles[relTrackPath] = struct{}{}
		}

		tracks = append(tracks, cueTracks...)

		log.Printf("Directory CUE processed OK '%s/%s'\n", s.Dir.Path, entry.Name())
	}
	return s.saveTracks(tracks)
}

func (s *DirectoryScanner) processAudioFiles() error {
	var tracks []*lib.Track
	for _, entry := range s.Entries {
		if entry.IsDir() {
			continue
		}
		if _, skip := s.SkipFiles[entry.Name()]; skip {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".mp3", ".flac", ".ogg", ".m4a", ".wav":
			// audio track - read tags
			track, err := scanAudioFile(s.FF, s.Dir.Path, entry.Name())
			if err != nil {
				return err
			}
			log.Printf("Tags scanned successfully '%s/%s'\n", s.Path, entry.Name())
			tracks = append(tracks, track)
		}
	}
	return s.saveTracks(tracks)
}

func (s *DirectoryScanner) saveTracks(tracks []*lib.Track) error {
	if len(tracks) == 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, track := range tracks {
		_, err = database.SaveTrackWithRelations(tx, s.Dir.ID, track)
		if err != nil {
			return err
		}
	}

	if err = database.UpdateDirectoryLastScan(tx, s.Dir.ID); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	log.Printf("Tracks saved successfully '%s'\n", s.Path)
	return nil
}

func ScanTracksInDir(path string) error {
	scanner, err := NewDirectoryScanner(path)
	if err != nil {
		return err
	}

	if !scanner.ShouldScan() {
		return nil
	}

	log.Printf("Starting to scan directory '%s'\n", path)

	if err := scanner.processCueFiles(); err != nil {
		log.Printf("scan cue error: %v", err)
		return err
	}

	if err := scanner.processAudioFiles(); err != nil {
		log.Printf("scan audio error: %v", err)
		return err
	}

	if err := scanner.processImages(); err != nil {
		log.Printf("scan images error: %v", err)
		return err
	}

	log.Printf("Directory scanned successfully '%s'\n", path)
	return nil
}
