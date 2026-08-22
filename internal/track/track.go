// Package track
package track

import (
	"bytes"
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
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

const sideFileName = "Side.txt"

func checkRelevantFile(name string) (bool, lib.ScanFileType) {
	if strings.EqualFold(name, sideFileName) {
		return true, lib.ScanFileTypeSide
	}
	if isArtistLogo(name) {
		return true, lib.ScanFileTypeLogo
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".cue", ".mp3", ".flac", ".ogg", ".m4a", ".ape", ".wav",
		".jpg", ".jpeg", ".png":
		return true, lib.ScanFileTypeAudio
	}
	return false, 0
}

func SyncDirectories() (map[string]any, error) {
	writeDB := database.Writer()
	cfg := config.GetConfig()

	// At first - search for music dirs and sync them with database
	var scanDirs = map[string]lib.DirScan{}
	for _, root := range cfg.RootPaths {
		log.Printf("Starting to read root directory '%s'\n", root)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// Covers is handled by DirectoryScanner of the parent directory
				// do not add it as a separate directory and do not scan anything inside it
				if strings.EqualFold(d.Name(), CoversDir) {
					return filepath.SkipDir
				}
				return nil
			}
			ok, fileType := checkRelevantFile(d.Name())
			if !ok {
				return nil
			}
			dirPath := filepath.Dir(path)

			// mtime max for files inside
			fileInfo, err := os.Stat(path)
			if err != nil {
				log.Printf("Loading file state error: %v", err)
				return err
			}
			fileMtime := fileInfo.ModTime().Unix()

			if dir, ok := scanDirs[dirPath]; ok {
				if fileType == lib.ScanFileTypeSide {
					dir.SideExists = true
				}
				// mtime max for files inside
				if fileMtime > dir.Mtime {
					dir.Mtime = fileMtime
					dir.LastScan = 0
				}
				scanDirs[dirPath] = dir
			} else {
				// Skip the root directory itself (which evaluates to ".")
				if dirPath == "." {
					return nil
				}
				scanDirs[dirPath] = lib.DirScan{
					Mtime:      fileMtime,
					LastScan:   0,
					SideExists: fileType == lib.ScanFileTypeSide,
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
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
		if dir, ok := scanDirs[path]; ok {
			if dirDB.Mtime != dir.Mtime || dirDB.SideExists != dir.SideExists {
				dirDB.Mtime = dir.Mtime
				dirDB.SideExists = dir.SideExists
				dirsUpdate[path] = dirDB
			}
			delete(scanDirs, path)
			delete(dirExists, path)
		}
	}

	// new dirs
	if len(scanDirs) > 0 {
		log.Printf("to add: %d\n", len(scanDirs))
		if err := database.SaveDirectories(writeDB, scanDirs); err != nil {
			return nil, err
		}
	}

	// modified dirs - update mtime
	if len(dirsUpdate) > 0 {
		log.Printf("to update: %d\n", len(dirsUpdate))
		if err := database.UpdateDirectories(writeDB, dirsUpdate); err != nil {
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
		"num":    len(scanDirs) + len(dirsUpdate),
		"add":    len(scanDirs),
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
	Path             string
	Dir              *lib.DirDB
	DB               *sql.DB
	Entries          []os.DirEntry
	FF               *ffmpeg.FFmpeg
	SkipFiles        map[string]struct{}
	Images           []lib.Image
	EmbeddedImages   []EmbeddedImage
	EmbeddedTrackIDs []int
	ScannedTrackIDs  []int
	ImageOrder       int
	Thumbnails       ThumbnailGenerator
	MainFrontFound   bool
	ArtistID         *int
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
		_, albumArtistID, _, _, err := database.SaveTrackWithRelations(tx, s.Dir.ID, track)
		if err != nil {
			return err
		}
		s.ScannedTrackIDs = append(s.ScannedTrackIDs, track.ID)
		if s.ArtistID == nil {
			s.ArtistID = &albumArtistID
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	log.Printf("Tracks saved successfully '%s'\n", s.Path)
	return nil
}

// artist relations
func (s *DirectoryScanner) processSideFile() error {
	var sidePath string
	var currentSideMtime int64

	for _, entry := range s.Entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(entry.Name(), sideFileName) {
			sidePath = filepath.Join(s.Path, entry.Name())
			info, err := entry.Info()
			if err != nil {
				return err
			}
			currentSideMtime = info.ModTime().Unix()
			break
		}
	}

	// Side.txt has not changed since the last time it was processed
	if currentSideMtime == s.Dir.SideMtime {
		return nil
	}

	artistName := filepath.Base(s.Path)

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	artistID, err := database.GetOrAddArtist(tx, artistName)
	if err != nil {
		return err
	}

	// get connections artist -> related_artist
	oldRelatedIDs, err := database.GetRelatedArtists(tx, artistID, database.RelationRelated)
	if err != nil {
		return err
	}

	// if Side.txt exists, we read it
	// if the file doesn't exist, newRelatedIDs will remain empty,
	// meaning all existing relationships are deleted
	var newRelatedIDs []int

	if sidePath != "" {
		data, err := os.ReadFile(sidePath)
		if err != nil {
			return err
		}
		// remove BOM
		data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
		if !utf8.Valid(data) {
			encodings := []encoding.Encoding{
				charmap.Windows1251,
				charmap.Windows1252,
				charmap.CodePage866,
				charmap.KOI8R,
			}
			for _, enc := range encodings {
				decoded, err := enc.NewDecoder().Bytes(data)
				if err != nil {
					continue
				}
				data = decoded
				break
			}
		}

		lines := strings.Split(string(data), "\n")

		newRelatedSet := make(map[int]struct{})

		for _, line := range lines {
			relatedName := strings.TrimSpace(line)
			if relatedName == "" {
				continue
			}
			if strings.EqualFold(artistName, relatedName) {
				continue
			}
			relatedID, err := database.GetOrAddArtist(tx, relatedName)
			if err != nil {
				return err
			}
			newRelatedSet[relatedID] = struct{}{}
		}

		newRelatedIDs = make([]int, 0, len(newRelatedSet))
		for relatedID := range newRelatedSet {
			newRelatedIDs = append(newRelatedIDs, relatedID)
		}

		sort.Ints(newRelatedIDs)
	}

	// create a set of existing connections
	oldSet := make(map[int]struct{}, len(oldRelatedIDs))
	for _, relatedID := range oldRelatedIDs {
		oldSet[relatedID] = struct{}{}
	}

	// create a set of new connections
	newSet := make(map[int]struct{}, len(newRelatedIDs))
	for _, relatedID := range newRelatedIDs {
		newSet[relatedID] = struct{}{}
	}

	isChanged := false

	// remove links that no longer exist in Side.txt
	for _, relatedID := range oldRelatedIDs {
		if _, exists := newSet[relatedID]; exists {
			continue
		}
		if err := database.DeleteArtistRelation(tx, artistID, relatedID); err != nil {
			return err
		}

		isChanged = true

		log.Printf("Artist relation removed: '%s' -> artist_id=%d\n", artistName, relatedID)
	}

	// adding new connections
	for _, relatedID := range newRelatedIDs {
		if _, exists := oldSet[relatedID]; exists {
			continue
		}
		if err := database.SaveArtistRelation(tx, artistID, relatedID); err != nil {
			return err
		}

		isChanged = true

		log.Printf("Artist relation saved: '%s' -> artist_id=%d\n", artistName, relatedID)
	}

	if err := database.UpdateDirectorySideMtime(tx, s.Dir.ID, currentSideMtime); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("Side.txt processed: artist '%s', changed=%v\n", artistName, isChanged)

	return nil
}

func (s *DirectoryScanner) markScanned() error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := database.UpdateDirectoryLastScan(tx, s.Dir.ID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

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

	// artist relations
	if err := scanner.processSideFile(); err != nil {
		log.Printf("scan side file error: %v", err)
		return err
	}

	if err := scanner.processImages(); err != nil {
		log.Printf("scan images error: %v", err)
		return err
	}

	if err := scanner.markScanned(); err != nil {
		return err
	}

	log.Printf("Directory scanned successfully '%s'\n", path)
	return nil
}
