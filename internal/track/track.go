// Package track
package track

import (
	"errors"
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
	"sonary/utils"
	"strings"
	"time"
)

func GetArtistKey(artistName string) string {
	return strings.ToLower(artistName)
}

func GetOrAddArtist(db database.DBTX, artistName string) (int, error) {
	ct := lib.GetImportContext(false)
	id, ok := ct.ArtistCache[GetArtistKey(artistName)]
	if !ok {
		artist, err := database.GetArtist(db, lib.ArtistsGetParams{Name: utils.Ptr(artistName)})
		if err != nil {
			if errors.Is(err, database.ErrArtistNotFound) {
				artistInput := &lib.Artist{Name: artistName}
				artist, err = database.SaveArtist(db, artistInput)
				if err != nil {
					return 0, err
				}
			} else {
				return 0, err
			}
		}
		id = artist.ID
		if ct.ArtistCache != nil {
			ct.ArtistCache[GetArtistKey(artistName)] = id
		}
	}
	return id, nil
}

func GetAlbumKey(artistName string, albumName string) string {
	return strings.ToLower(artistName + "|" + albumName)
}

func GetOrAddAlbum(db database.DBTX, artistID int, track *lib.Track) (int, error) {
	ct := lib.GetImportContext(false)
	id, ok := ct.AlbumCache[GetAlbumKey(track.Artist, track.Album)]
	if !ok {
		album, err := database.GetAlbum(db, lib.AlbumsGetParams{
			ArtistID: utils.Ptr(artistID), Title: utils.Ptr(track.Album),
		})
		if err != nil {
			if errors.Is(err, database.ErrAlbumNotFound) {
				artistInput := &lib.Album{
					ID:       track.ID,
					ArtistID: artistID,
					Title:    track.Album,
					Year:     track.Year,
				}
				album, err = database.SaveAlbum(db, artistInput)
				if err != nil {
					return 0, err
				}
			} else {
				return 0, err
			}
		}
		id = album.ID
		if ct.AlbumCache != nil {
			ct.AlbumCache[GetAlbumKey(track.Artist, track.Album)] = id
		}
	}
	return id, nil
}

func SyncDirectories() (map[string]any, error) {
	writeDB := database.Writer()
	cfg := config.GetConfig()
	root := cfg.RootPath

	log.Printf("Starting to read root directory '%s'\n", root)
	// At first - search for music dirs and sync them with database
	var musicDirs = map[string]lib.DirScan{}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		switch ext {
		case ".cue", ".mp3", ".flac", ".ogg", ".m4a", ".ape", ".wav":
			dirPath := filepath.Dir(path)
			// Calculate the path relative to the root directory
			relDirPath, err := filepath.Rel(root, dirPath)
			if err != nil {
				log.Printf("Calculate relative path error: %v", err)
				return err
			}
			if dir, ok := musicDirs[relDirPath]; ok {
				// mtime max for files inside
				fileInfo, err := os.Stat(path)
				if err != nil {
					log.Printf("Loading file state error: %v", err)
					return err
				}
				fileMtime := fileInfo.ModTime().Unix()
				if fileMtime > dir.Mtime {
					musicDirs[relDirPath] = lib.DirScan{
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
				if relDirPath == "." {
					return nil
				}
				musicDirs[relDirPath] = lib.DirScan{
					Mtime:    dirInfo.ModTime().Unix(),
					LastScan: 0,
				}
			}
		}

		return nil
	})

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

func ScanTracksInDir(path string) error {
	writeDB := database.Writer()
	cfg := config.GetConfig()
	ff := ffmpeg.NewFFmpeg()
	root := cfg.RootPath

	dir, err := database.GetDirectory(writeDB, path)
	if err != nil {
		log.Printf("Get directory data error: %v", path)
		return err
	}

	fullPath := filepath.Join(root, dir.Path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		log.Printf("Loading directory content error: %v", path)
		return err
	}
	if dir.LastScan != 0 {
		log.Printf("Directory skipping. '%s'\n", path)
		return nil
	}
	log.Printf("Starting to sync directory '%s'\n", path)
	skipFiles := make(map[string]struct{})
	for _, entry := range entries {
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
		tracks, er := scanCue(ff, root, dir.Path, entry.Name())
		if er != nil {
			log.Printf("Scan CUE error: %v", path)
			return er
		}
		log.Printf("CUE scanned successfully '%s'\n", path)

		skipFiles[entry.Name()] = struct{}{}

		tx, er := writeDB.Begin()
		if er != nil {
			return er
		}
		for _, track := range tracks {
			relTrackPath, er := filepath.Rel(dir.Path, track.Path)
			if er != nil {
				return er
			}
			skipFiles[relTrackPath] = struct{}{}
			albumArtist := track.AlbumArtist
			if albumArtist == "" {
				albumArtist = track.Artist
			}
			if albumArtist == "" {
				albumArtist = "Unknown Artist"
			}
			albumArtistID, er := GetOrAddArtist(tx, albumArtist)
			if er != nil {
				tx.Rollback()
				return er
			}
			albumID, er := GetOrAddAlbum(tx, albumArtistID, track)
			if er != nil {
				tx.Rollback()
				return er
			}
			track.AlbumID = albumID
			trackArtist := track.Artist
			if trackArtist == "" {
				trackArtist = track.AlbumArtist
			}
			if trackArtist == "" {
				trackArtist = "Unknown Artist"
			}
			artistID, er := GetOrAddArtist(tx, trackArtist)
			if er != nil {
				tx.Rollback()
				return er
			}
			_, er = database.SaveTrack(tx, dir.ID, artistID, track)
			if er != nil {
				tx.Rollback()
				return er
			}
			log.Printf("Track saved OK '%s' : '%s'\n", relTrackPath, track.Title)
		}
		err = database.UpdateDirectoryLastScan(tx, dir.ID)
		if err != nil {
			tx.Rollback()
			return err
		}
		err = tx.Commit()
		if err != nil {
			tx.Rollback()
			return err
		}
		log.Printf("Directory CUE processed OK '%s/%s'\n", dir.Path, entry.Name())
	}
	tx, err := writeDB.Begin()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, skip := skipFiles[entry.Name()]; skip {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".mp3", ".flac", ".ogg", ".m4a", ".wav":
			// audio track - read tags
			track, er := scanAudioFile(ff, root, dir.Path, entry.Name())
			if er != nil {
				return er
			}
			log.Printf("Tags scanned successfully '%s/%s'\n", path, entry.Name())
			albumArtist := track.AlbumArtist
			if albumArtist == "" {
				albumArtist = track.Artist
			}
			if albumArtist == "" {
				albumArtist = "Unknown Artist"
			}
			albumArtistID, er := GetOrAddArtist(tx, albumArtist)
			if er != nil {
				tx.Rollback()
				return er
			}
			albumID, er := GetOrAddAlbum(tx, albumArtistID, track)
			if er != nil {
				tx.Rollback()
				return er
			}
			track.AlbumID = albumID
			trackArtist := track.Artist
			if trackArtist == "" {
				trackArtist = track.AlbumArtist
			}
			if trackArtist == "" {
				trackArtist = "Unknown Artist"
			}
			artistID, er := GetOrAddArtist(tx, trackArtist)
			if er != nil {
				tx.Rollback()
				return er
			}
			_, er = database.SaveTrack(tx, dir.ID, artistID, track)
			if er != nil {
				tx.Rollback()
				return er
			}
			log.Printf("Track saved OK '%s/%s'\n", dir.Path, entry.Name())
		}
	}
	err = database.UpdateDirectoryLastScan(tx, dir.ID)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}
	log.Printf("Directory scanned successfully '%s'\n", path)
	return nil
}
