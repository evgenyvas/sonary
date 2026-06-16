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
	"sonary/internal/database"
	"sonary/internal/ffmpeg"
	"sonary/internal/lib"
	"sonary/utils"
	"strings"
	"time"
)

const batchSize = 500

func GetDirectories(db *sql.DB) (map[string]lib.DirDB, error) {
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

func SyncDirectories(db *sql.DB, root string) (int, error) {
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
				return err
			}
			if dir, ok := musicDirs[relDirPath]; ok {
				// mtime max for files inside
				fileInfo, err := os.Stat(path)
				if err != nil {
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
		return 0, err
	}

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
		fmt.Printf("to add: %d\n", len(musicDirs))
		if err := SaveDirectories(db, musicDirs); err != nil {
			return 0, err
		}
	}

	// modified dirs - update mtime
	if len(dirsUpdate) > 0 {
		fmt.Printf("to update: %d\n", len(dirsUpdate))
		if err := UpdateDirectoriesMtime(db, dirsUpdate); err != nil {
			return 0, err
		}
	}

	// dirs to delete
	if len(dirExists) > 0 {
		fmt.Printf("to delete: %d\n", len(dirExists))
		if err := DeleteDirectories(db, dirExists); err != nil {
			return 0, err
		}
	}

	// TODO: если директория поменялась
	//DELETE FROM tracks
	//WHERE directory = ?

	//DELETE FROM albums
	//WHERE NOT EXISTS (
	//SELECT 1
	//FROM tracks
	//WHERE tracks.album_id = albums.id
	//);

	return len(musicDirs) + len(dirsUpdate), nil
}

func FormatTrackDuration(duration time.Duration) string {
	minutes := int(duration / time.Minute)
	seconds := int((duration % time.Minute) / time.Second)
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

type ImportContext struct {
	ArtistCache map[string]int
	AlbumCache  map[string]int
}

func (c *ImportContext) GetArtistKey(artistName string) string {
	return strings.ToLower(artistName)
}

func (c *ImportContext) GetOrAddArtist(db database.DBTX, artistName string) (int, error) {
	id, ok := c.ArtistCache[c.GetArtistKey(artistName)]
	if !ok {
		artist, err := GetArtist(db, artistName)
		if err != nil {
			return 0, err
		}
		if artist == nil {
			artistInput := &lib.Artist{Name: artistName}
			artist, err = SaveArtist(db, artistInput)
			if err != nil {
				return 0, err
			}
		}
		id = artist.ID
		c.ArtistCache[c.GetArtistKey(artistName)] = id
	}
	return id, nil
}

func (c *ImportContext) GetAlbumKey(artistName string, albumName string) string {
	return strings.ToLower(artistName + "|" + albumName)
}

func (c *ImportContext) GetOrAddAlbum(db database.DBTX, artistID int, track *lib.Track) (int, error) {
	id, ok := c.AlbumCache[c.GetAlbumKey(track.Artist, track.Album)]
	if !ok {
		album, err := GetAlbum(db, artistID, track.Album)
		if err != nil {
			return 0, err
		}
		if album == nil {
			artistInput := &lib.Album{
				ID:       track.ID,
				ArtistID: artistID,
				Title:    track.Album,
				Year:     track.Year,
			}
			album, err = SaveAlbum(db, artistInput)
			if err != nil {
				return 0, err
			}
		}
		id = album.ID
		c.AlbumCache[c.GetAlbumKey(track.Artist, track.Album)] = id
	}
	return id, nil
}

func ScanTracks(db *sql.DB, root string, progress *lib.Progress) error {
	dirs, err := GetDirectories(db)
	if err != nil {
		return err
	}

	ff := ffmpeg.NewFFmpeg()
	ct := &ImportContext{
		ArtistCache: map[string]int{},
		AlbumCache:  map[string]int{},
	}

	for path, dir := range dirs {
		fullPath := filepath.Join(root, path)
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return err
		}
		if dir.LastScan != 0 {
			continue
		}
		fmt.Println(dir.Path)
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
			tracks, er := scanCue(ff, root, path, entry.Name())
			if er != nil {
				return er
			}

			skipFiles[entry.Name()] = struct{}{}

			tx, er := db.Begin()
			if er != nil {
				return er
			}
			for _, track := range tracks {
				relTrackPath, er := filepath.Rel(path, track.Path)
				if er != nil {
					return er
				}
				skipFiles[relTrackPath] = struct{}{}
				albumArtist := track.AlbumArtist
				if albumArtist == "" {
					albumArtist = track.Artist
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
				artistID, er := ct.GetOrAddArtist(tx, track.Artist)
				if er != nil {
					tx.Rollback()
					return er
				}
				SaveTrack(tx, dir.ID, artistID, track)
				fmt.Printf("%#v\n", *track)
			}
			UpdateDirectoryLastScan(tx, dir.ID)
			err = tx.Commit()
			if err != nil {
				tx.Rollback()
				return err
			}
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
				track, er := scanAudioFile(ff, root, path, entry.Name())
				if er != nil {
					return er
				}
				albumArtist := track.AlbumArtist
				if albumArtist == "" {
					albumArtist = track.Artist
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
				artistID, er := ct.GetOrAddArtist(tx, track.Artist)
				if er != nil {
					tx.Rollback()
					return er
				}
				SaveTrack(tx, dir.ID, artistID, track)
				fmt.Printf("%#v\n", track)
			}
		}
		UpdateDirectoryLastScan(tx, dir.ID)
		err = tx.Commit()
		if err != nil {
			tx.Rollback()
			return err
		}
		progress.Processed.Add(1)
		break
	}

	return nil
}

func ScanLibrary(db *sql.DB, root string) error {
	num, err := SyncDirectories(db, root)
	if err != nil {
		return err
	}

	progress := lib.Progress{
		Total: num,
	}

	ScanTracks(db, root, &progress)

	return nil
}
