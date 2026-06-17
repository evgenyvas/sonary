// Package track
package track

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sonary/internal/config"
	"sonary/internal/database"
	"sonary/internal/ffmpeg"
	"sonary/internal/lib"
	"sonary/utils"
	"strings"
	"time"
)

const batchSize = 500

func GetDirectories(db database.DBTX) (map[string]lib.DirDB, error) {
	query := `
		SELECT id, path, mtime, last_scan
		FROM directories
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dirs = map[string]lib.DirDB{}

	for rows.Next() {
		var d lib.DirDB
		err := rows.Scan(&d.ID, &d.Path, &d.Mtime, &d.LastScan)
		if err != nil {
			return nil, err
		}
		dirs[d.Path] = d
	}

	return dirs, nil
}

func GetDirectory(db database.DBTX, path string) (*lib.DirDB, error) {
	query := `
		SELECT id, mtime, last_scan
		FROM directories WHERE path = ?
	`

	dir := &lib.DirDB{Path: path}

	err := db.QueryRow(query, path).Scan(&dir.ID, &dir.Mtime, &dir.LastScan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return dir, nil
}

func SaveDirectories(db *sql.DB, dirs map[string]lib.DirScan) error {
	for _, chunk := range utils.ChunkMap(dirs, batchSize) {
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			return err
		}

		stmt, err := tx.Prepare(
			"INSERT INTO directories (path, mtime, last_scan) VALUES (?, ?, ?)",
		)
		if err != nil {
			tx.Rollback()
			return err
		}

		for dir, dirScan := range chunk {
			_, err := stmt.Exec(dir, dirScan.Mtime, dirScan.LastScan)
			if err != nil {
				stmt.Close()
				tx.Rollback()
				return err
			}
		}

		if err := stmt.Close(); err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}

func UpdateDirectoriesMtime(db *sql.DB, dirs map[string]lib.DirDB) error {
	for _, chunk := range utils.ChunkMap(dirs, batchSize) {
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			return err
		}

		stmt, err := tx.Prepare(
			"UPDATE directories SET mtime = ?, last_scan = 0 WHERE id = ?",
		)
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, dirDB := range chunk {
			res, err := stmt.Exec(dirDB.Mtime, dirDB.ID)
			if err != nil {
				stmt.Close()
				tx.Rollback()
				return err
			}

			n, _ := res.RowsAffected()
			if n == 0 {
				log.Printf("directory id=%d not found", dirDB.ID)
			}
		}

		stmt.Close()

		if err := tx.Commit(); err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}

var ErrDirectoryNotFound = errors.New("directory not found")

func UpdateDirectoryLastScan(db database.DBTX, dirID int) error {
	dateNow := time.Now().Unix()

	res, err := db.Exec(
		`UPDATE directories SET last_scan = ? WHERE id = ?`,
		dateNow, dirID)
	if err != nil {
		return fmt.Errorf("update directory last_scan: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows: %w", err)
	}
	if n == 0 {
		return ErrDirectoryNotFound
	}

	return nil
}

func DeleteDirectories(db *sql.DB, dirs map[string]lib.DirDB) error {
	for _, chunk := range utils.ChunkMap(dirs, batchSize) {
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			return err
		}

		ids := make([]any, 0, len(chunk))
		placeholders := make([]string, 0, len(chunk))

		for _, dirDB := range chunk {
			ids = append(ids, dirDB.ID)
			placeholders = append(placeholders, "?")
		}

		query := fmt.Sprintf(
			"DELETE FROM directories WHERE id IN (%s)",
			strings.Join(placeholders, ","),
		)

		res, err := tx.Exec(query, ids...)
		if err != nil {
			tx.Rollback()
			return err
		}

		affected, err := res.RowsAffected()
		if err != nil {
			tx.Rollback()
			return err
		}

		if affected != int64(len(chunk)) {
			tx.Rollback()
			return fmt.Errorf(
				"deleted %d rows, expected %d",
				affected,
				len(chunk),
			)
		}

		// delete tracks
		query = fmt.Sprintf(
			"DELETE FROM tracks WHERE directory_id IN (%s)",
			strings.Join(placeholders, ","),
		)

		_, err = tx.Exec(query, ids...)
		if err != nil {
			tx.Rollback()
			return err
		}

		// delete tracks
		_, err = tx.Exec(`DELETE FROM albums
			WHERE NOT EXISTS (
			SELECT 1
			FROM tracks
			WHERE tracks.album_id = albums.id
		)`)
		if err != nil {
			tx.Rollback()
			return err
		}

		// delete artists
		_, err = tx.Exec(`DELETE FROM artists
			WHERE NOT EXISTS (
			SELECT 1
			FROM albums
			WHERE albums.artist_id = artists.id
		)`)
		if err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}

func GetArtist(db database.DBTX, artistName string) (*lib.ArtistDB, error) {
	query := `SELECT id FROM artists WHERE name = ?`

	artist := &lib.ArtistDB{Name: artistName}

	err := db.QueryRow(query, artistName).Scan(&artist.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return artist, nil
}

func SaveArtist(db database.DBTX, artistInput *lib.Artist) (*lib.ArtistDB, error) {
	artist := &lib.ArtistDB{}

	err := db.QueryRow(`
		INSERT INTO artists (name)
		VALUES (?)
		RETURNING id, name`,
		artistInput.Name,
	).Scan(&artist.ID, &artist.Name)

	if err != nil {
		return nil, err
	}

	return artist, nil
}

func GetAlbum(db database.DBTX, artistID int, albumName string) (*lib.AlbumDB, error) {
	query := `SELECT id, year FROM albums WHERE artist_id = ? AND title = ?`

	album := &lib.AlbumDB{
		ArtistID: artistID,
		Title:    albumName,
	}

	err := db.QueryRow(query, artistID, albumName).Scan(&album.ID, &album.Year)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return album, nil
}

func SaveAlbum(db database.DBTX, albumInput *lib.Album) (*lib.AlbumDB, error) {
	album := &lib.AlbumDB{}

	err := db.QueryRow(`
		INSERT INTO albums (artist_id, title, year)
		VALUES (?, ?, ?)
		RETURNING id, artist_id, title, year`,
		albumInput.ArtistID, albumInput.Title, albumInput.Year,
	).Scan(&album.ID, &album.ArtistID, &album.Title, &album.Year)

	if err != nil {
		return nil, err
	}

	return album, nil
}

func SaveTrack(db database.DBTX, dirID int, artistID int, track *lib.Track) (*lib.Track, error) {
	err := db.QueryRow(`
		INSERT INTO tracks (
			album_id, directory_id, artist_id, path, file_type, title, year,
			genre, track_number, duration, lyrics, is_cue, cue_file, cue_offset
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		track.AlbumID, dirID, artistID, track.Path, track.FileType, track.Title,
		track.Year, track.Genre, track.TrackNumber, track.Duration, track.Lyrics,
		track.IsCue, track.CueFile, track.CueOffset,
	).Scan(&track.ID)
	if err != nil {
		return nil, err
	}
	return track, nil
}

func SyncDirectories() (map[string]any, error) {
	db := database.GetDB()
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

	dirExists, err := GetDirectories(db)
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
		if err := SaveDirectories(db, musicDirs); err != nil {
			return nil, err
		}
	}

	// modified dirs - update mtime
	if len(dirsUpdate) > 0 {
		log.Printf("to update: %d\n", len(dirsUpdate))
		if err := UpdateDirectoriesMtime(db, dirsUpdate); err != nil {
			return nil, err
		}
	}

	// dirs to delete
	if len(dirExists) > 0 {
		log.Printf("to delete: %d\n", len(dirExists))
		if err := DeleteDirectories(db, dirExists); err != nil {
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
	db := database.GetDB()
	cfg := config.GetConfig()
	ct := GetImportContext(0)
	ff := ffmpeg.NewFFmpeg()
	root := cfg.RootPath

	dir, err := GetDirectory(db, path)
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

		// parse CUE
		tracks, er := scanCue(ff, root, dir.Path, entry.Name())
		if er != nil {
			log.Printf("Scan CUE error: %v", path)
			return er
		}
		log.Printf("CUE scanned successfully '%s'\n", path)

		skipFiles[entry.Name()] = struct{}{}

		tx, er := db.Begin()
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
			albumArtistID, er := ct.GetOrAddArtist(tx, albumArtist)
			if er != nil {
				tx.Rollback()
				return er
			}
			albumID, er := ct.GetOrAddAlbum(tx, albumArtistID, track)
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
			artistID, er := ct.GetOrAddArtist(tx, trackArtist)
			if er != nil {
				tx.Rollback()
				return er
			}
			_, er = SaveTrack(tx, dir.ID, artistID, track)
			if er != nil {
				tx.Rollback()
				return er
			}
			log.Printf("Track saved OK '%s' : '%s'\n", relTrackPath, track.Title)
		}
		err = UpdateDirectoryLastScan(tx, dir.ID)
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
	tx, err := db.Begin()
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
			albumArtistID, er := ct.GetOrAddArtist(tx, albumArtist)
			if er != nil {
				tx.Rollback()
				return er
			}
			albumID, er := ct.GetOrAddAlbum(tx, albumArtistID, track)
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
			artistID, er := ct.GetOrAddArtist(tx, trackArtist)
			if er != nil {
				tx.Rollback()
				return er
			}
			_, er = SaveTrack(tx, dir.ID, artistID, track)
			if er != nil {
				tx.Rollback()
				return er
			}
			log.Printf("Track saved OK '%s/%s'\n", dir.Path, entry.Name())
		}
	}
	err = UpdateDirectoryLastScan(tx, dir.ID)
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
