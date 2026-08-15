package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"
	"sonary/internal/lib"
	"sonary/utils"
	"strings"
	"time"
)

const batchSize = 500

func GetDirectories(db DBTX) (map[string]lib.DirDB, error) {
	query := `
		SELECT id, path, mtime, last_scan, side_exists, side_mtime
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
		err := rows.Scan(&d.ID, &d.Path, &d.Mtime, &d.LastScan,
			&d.SideExists, &d.SideMtime)
		if err != nil {
			return nil, err
		}
		dirs[d.Path] = d
	}

	return dirs, nil
}

func GetDirectory(db DBTX, path string) (*lib.DirDB, error) {
	query := `
		SELECT id, mtime, last_scan, side_exists, side_mtime
		FROM directories WHERE path = ?
	`

	dir := &lib.DirDB{Path: path}

	err := db.QueryRow(query, path).Scan(&dir.ID, &dir.Mtime, &dir.LastScan,
		&dir.SideExists, &dir.SideMtime)
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

		stmt, err := tx.Prepare(`
			INSERT INTO directories (path, mtime, last_scan, side_exists, side_mtime)
			VALUES (?, ?, ?, ?, ?)
		`)
		if err != nil {
			tx.Rollback()
			return err
		}

		for dir, dirScan := range chunk {
			_, err := stmt.Exec(dir, dirScan.Mtime, dirScan.LastScan,
				dirScan.SideExists, dirScan.SideMtime)
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

func UpdateDirectories(db *sql.DB, dirs map[string]lib.DirDB) error {
	for _, chunk := range utils.ChunkMap(dirs, batchSize) {
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			return err
		}

		// last_scan = 0 so directory will be rescanned
		stmt, err := tx.Prepare(
			"UPDATE directories SET mtime = ?, last_scan = 0, side_exists = ? WHERE id = ?",
		)
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, dirDB := range chunk {
			res, err := stmt.Exec(dirDB.Mtime, dirDB.SideExists, dirDB.ID)
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

func UpdateDirectoryLastScan(db DBTX, dirID int) error {
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

func UpdateDirectorySideMtime(db DBTX, dirID int, sideMtime int64) error {
	res, err := db.Exec(
		`UPDATE directories SET side_mtime = ? WHERE id = ?`,
		sideMtime, dirID)
	if err != nil {
		return fmt.Errorf("update directory side_mtime: %w", err)
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

		// delete tracks
		query := fmt.Sprintf(
			"DELETE FROM tracks WHERE directory_id IN (%s)",
			strings.Join(placeholders, ","),
		)

		_, err = tx.Exec(query, ids...)
		if err != nil {
			tx.Rollback()
			return err
		}

		// delete albums
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
		) AND NOT EXISTS (
			SELECT 1
			FROM tracks
			WHERE tracks.artist_id = artists.id
		)`)
		if err != nil {
			tx.Rollback()
			return err
		}

		// delete directories
		query = fmt.Sprintf(
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

func SaveArtist(db DBTX, artistInput *lib.Artist) (*lib.ArtistDB, error) {
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

type RelationDirection int

const (
	RelationBoth RelationDirection = iota
	RelationRelated
	RelationArtist
)

func GetRelatedArtists(db DBTX, artistID int, direction RelationDirection) ([]int, error) {
	var query string
	var args []any

	switch direction {
	case RelationRelated:
		query = `
			SELECT related_artist_id
			FROM artist_relations
			WHERE artist_id = ?
			ORDER BY related_artist_id
		`
		args = []any{artistID}

	case RelationArtist:
		query = `
			SELECT artist_id
			FROM artist_relations
			WHERE related_artist_id = ?
			ORDER BY artist_id
		`
		args = []any{artistID}

	case RelationBoth:
		query = `
			SELECT related_artist_id AS artist_id
			FROM artist_relations
			WHERE artist_id = ?

			UNION

			SELECT artist_id
			FROM artist_relations
			WHERE related_artist_id = ?

			ORDER BY artist_id
		`
		args = []any{artistID, artistID}

	default:
		return nil, fmt.Errorf("unknown relation direction: %d", direction)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artistIDs []int

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		artistIDs = append(artistIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return artistIDs, nil
}

func SaveArtistRelation(db DBTX, artistID, relatedArtistID int) error {
	_, err := db.Exec(`
		INSERT OR IGNORE INTO artist_relations
			(artist_id, related_artist_id)
		VALUES (?, ?)
	`, artistID, relatedArtistID)

	return err
}

func DeleteArtistRelation(db DBTX, artistID, relatedArtistID int) error {
	_, err := db.Exec(`
		DELETE FROM artist_relations
		WHERE artist_id = ? AND related_artist_id = ?
	`, artistID, relatedArtistID)

	return err
}

func SaveAlbum(db DBTX, albumInput *lib.Album) (*lib.AlbumDB, error) {
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

func GetArtistKey(artistName string) string {
	return strings.ToLower(artistName)
}

func GetOrAddArtist(db DBTX, artistName string) (int, error) {
	ct := lib.GetImportContext()
	id, ok := ct.ArtistCache[GetArtistKey(artistName)]
	if !ok {
		artist, err := GetArtist(db, lib.ArtistsGetParams{Name: utils.Ptr(artistName)})
		if err != nil {
			if errors.Is(err, ErrArtistNotFound) {
				artistInput := &lib.Artist{Name: artistName}
				artist, err = SaveArtist(db, artistInput)
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

func GetOrAddAlbum(db DBTX, artistID int, track *lib.Track) (int, error) {
	ct := lib.GetImportContext()
	id, ok := ct.AlbumCache[GetAlbumKey(track.Artist, track.Album)]
	if !ok {
		album, err := GetAlbum(db, lib.AlbumsGetParams{
			ArtistID: utils.Ptr(artistID), Title: utils.Ptr(track.Album),
		})
		if err != nil {
			if errors.Is(err, ErrAlbumNotFound) {
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

func SaveTrackWithRelations(db DBTX, dirID int, track *lib.Track) (int, int, int, *lib.Track, error) {
	albumArtist := utils.FirstNonEmpty(
		track.AlbumArtist,
		track.Artist,
		"Unknown Artist",
	)
	albumArtistID, err := GetOrAddArtist(db, albumArtist)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	albumID, err := GetOrAddAlbum(db, albumArtistID, track)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	track.AlbumID = albumID

	trackArtist := utils.FirstNonEmpty(
		track.Artist,
		track.AlbumArtist,
		"Unknown Artist",
	)
	artistID, err := GetOrAddArtist(db, trackArtist)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	t, err := SaveTrack(db, dirID, artistID, track)
	return albumID, albumArtistID, artistID, t, err
}

func SaveTrack(db DBTX, dirID int, artistID int, track *lib.Track) (*lib.Track, error) {
	err := db.QueryRow(`
		INSERT INTO tracks (
			album_id, directory_id, artist_id, path, file_type, title, year,
			genre, track_number, duration, has_pregap, pregap_duration, lyrics,
			is_cue, cue_file, cue_offset
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		track.AlbumID, dirID, artistID, track.Path, track.FileType, track.Title,
		track.Year, track.Genre, track.TrackNumber, track.Duration, track.HasPregap,
		track.PregapDuration, track.Lyrics, track.IsCue, track.CueFile, track.CueOffset,
	).Scan(&track.ID)
	if err != nil {
		return nil, err
	}
	return track, nil
}

func SaveImage(db DBTX, dirID int, img *lib.Image) (*lib.Image, error) {
	err := db.QueryRow(`
		INSERT INTO images (
			directory_id, artist_id, path, type, format, sort_order, width, height, size, mtime
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		dirID, img.ArtistID, img.Path, img.Type, img.Format, img.Order, img.Width, img.Height, img.Size, img.Mtime,
	).Scan(&img.ID)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func getImages(db DBTX, params lib.ImagesGetParams) ([]lib.Image, error) {
	if len(params.DirectoryIDs) == 0 && len(params.ArtistIDs) == 0 {
		return []lib.Image{}, nil
	}

	var sb strings.Builder
	var conditions []string
	var args []any

	sb.WriteString(`
		SELECT id, directory_id, artist_id, path, type, format,
			sort_order, width, height, size, mtime
		FROM images
	`)

	// Directory IDs
	if len(params.DirectoryIDs) > 0 {
		placeholders := make([]string, len(params.DirectoryIDs))

		for i, id := range params.DirectoryIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}

		conditions = append(
			conditions,
			"directory_id IN ("+strings.Join(placeholders, ",")+")",
		)
	}

	// Artist IDs
	if len(params.ArtistIDs) > 0 {
		placeholders := make([]string, len(params.ArtistIDs))

		for i, id := range params.ArtistIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}

		conditions = append(
			conditions,
			"artist_id IN ("+strings.Join(placeholders, ",")+")",
		)
	}

	// Image type
	if params.Type != nil {
		conditions = append(conditions, "type = ?")
		args = append(args, *params.Type)
	}

	if len(conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	sb.WriteString(`
		ORDER BY directory_id, sort_order, id
	`)

	rows, err := db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := make([]lib.Image, 0)

	for rows.Next() {
		var img lib.Image

		if err := rows.Scan(
			&img.ID, &img.DirectoryID, &img.ArtistID, &img.Path, &img.Type, &img.Format,
			&img.Order, &img.Width, &img.Height, &img.Size, &img.Mtime,
		); err != nil {
			return nil, err
		}

		images = append(images, img)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return images, nil
}

func GetImagesFlat(db DBTX, params lib.ImagesGetParams) ([]lib.Image, error) {
	return getImages(db, params)
}

func GetImages(db DBTX, params lib.ImagesGetParams) (map[int][]lib.Image, error) {
	images, err := getImages(db, params)
	if err != nil {
		return nil, err
	}

	result := make(map[int][]lib.Image)

	for _, img := range images {
		var key int

		switch params.GroupBy {
		case lib.ImageGroupByArtist:
			if img.ArtistID == nil {
				continue
			}
			key = *img.ArtistID

		case lib.ImageGroupByDirectory:
			key = img.DirectoryID

		default:
			return nil, fmt.Errorf("unsupported image group: %d", params.GroupBy)
		}

		result[key] = append(result[key], img)
	}

	return result, nil
}

func GetAlbumDirectories(db DBTX, albumIDs []int) (map[int][]int, error) {
	if len(albumIDs) == 0 {
		return make(map[int][]int), nil
	}

	placeholders := make([]string, len(albumIDs))
	args := make([]any, len(albumIDs))

	for i, id := range albumIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `
		SELECT DISTINCT album_id, directory_id
		FROM tracks
		WHERE album_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY album_id, directory_id
	`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int][]int)

	for rows.Next() {
		var albumID int
		var directoryID int
		if err := rows.Scan(&albumID, &directoryID); err != nil {
			return nil, err
		}
		result[albumID] = append(result[albumID], directoryID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func DeleteImage(db DBTX, imageID int) error {
	_, err := db.Exec(`DELETE FROM images WHERE id = ?`, imageID)
	return err
}

var ErrTrackNotFound = errors.New("track not found")

func GetTrack(db *sql.DB, ID int) (*lib.TrackDB, error) {
	tracks, _, err := GetTracks(db, lib.TracksGetParams{ID: utils.Ptr(ID)})
	if err != nil {
		return nil, err
	}
	if len(tracks) != 1 {
		return nil, ErrTrackNotFound
	}
	return &tracks[0], nil
}

func GetTracks(db *sql.DB, params lib.TracksGetParams) ([]lib.TrackDB, bool, error) {
	var sb strings.Builder

	sb.WriteString(`
		SELECT t.id, t.path, t.file_type, t.title, ar.name, t.artist_id, alr.name,
				t.year, t.genre, al.title, t.album_id, t.track_number, t.duration,
				t.has_pregap, t.pregap_duration, t.lyrics, t.is_cue, t.cue_file,
				t.cue_offset, t.is_like, t.directory_id
		FROM tracks t
		LEFT JOIN albums al ON al.id = t.album_id
		LEFT JOIN artists ar ON ar.id = t.artist_id
		LEFT JOIN artists alr ON alr.id = al.artist_id
	`)

	var (
		conditions []string
		args       []any
	)

	// ID in priority
	if params.ID != nil {
		conditions = append(conditions, "t.id = ?")
		args = append(args, *params.ID)
	} else {
		if params.AlbumID != nil {
			conditions = append(conditions, "t.album_id = ?")
			args = append(args, *params.AlbumID)
		}
		if params.ArtistID != nil {
			conditions = append(conditions, "t.artist_id = ?")
			args = append(args, *params.ArtistID)
		}
		if params.Like != nil {
			conditions = append(conditions, "t.is_like = ?")
			args = append(args, *params.Like)
		}
		if params.NoAlbum {
			conditions = append(conditions, "al.artist_id != t.artist_id")
		}
	}

	if len(conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	// sort
	if params.Random {
		sb.WriteString(" ORDER BY RANDOM() ")
	} else if params.AlbumID != nil {
		sb.WriteString(" ORDER BY t.track_number ASC, t.id ASC ")
	} else {
		sb.WriteString(" ORDER BY t.id ASC ")
	}

	// limit / offset
	if params.ID != nil {
		sb.WriteString(" LIMIT 1")
	} else if params.Limit > 0 {
		sb.WriteString(" LIMIT ? ")
		args = append(args, params.Limit+1)
		if params.Page != nil && *params.Page > 0 {
			offset := (*params.Page - 1) * params.Limit
			sb.WriteString(" OFFSET ? ")
			args = append(args, offset)
		}
	}

	rows, err := db.Query(sb.String(), args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer rows.Close()

	var tracks []lib.TrackDB

	var countRows int
	for rows.Next() {
		var t lib.TrackDB

		err := rows.Scan(&t.ID, &t.Path, &t.FileType, &t.Title, &t.Artist, &t.ArtistID,
			&t.AlbumArtist, &t.Year, &t.Genre, &t.Album, &t.AlbumID, &t.TrackNumber,
			&t.Duration, &t.HasPregap, &t.PregapDuration, &t.Lyrics, &t.IsCue, &t.CueFile,
			&t.CueOffset, &t.IsLike, &t.DirectoryID)
		if err != nil {
			return nil, false, err
		}

		tracks = append(tracks, t)

		countRows++
	}

	if params.Limit > 0 && len(tracks) > params.Limit {
		tracks = slices.Delete(tracks, len(tracks)-1, len(tracks))
	}
	return tracks, params.Limit > 0 && countRows > params.Limit, rows.Err()
}

var ErrNothingToUpdate = errors.New("nothing to update in track")

func UpdateTrack(db DBTX, trackID int, params lib.TrackUpdateParams) error {
	sets := []string{}
	args := []any{}

	add := func(column string, value any) {
		sets = append(sets, column+" = ?")
		args = append(args, value)
	}

	if params.Like != nil {
		add("is_like", *params.Like)
	}

	if len(sets) == 0 {
		return ErrNothingToUpdate
	}

	query := fmt.Sprintf(
		`UPDATE tracks SET %s WHERE id = ?`,
		strings.Join(sets, ", "))
	args = append(args, trackID)
	res, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update track: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows: %w", err)
	}
	if n == 0 {
		return ErrTrackNotFound
	}

	return nil
}

var ErrArtistNotFound = errors.New("artist not found")

func GetArtist(db DBTX, params lib.ArtistsGetParams) (*lib.ArtistDB, error) {
	artists, _, err := GetArtists(db, params)
	if err != nil {
		return nil, err
	}
	if len(artists) != 1 {
		return nil, ErrArtistNotFound
	}
	return &artists[0], nil
}

func GetArtists(db DBTX, params lib.ArtistsGetParams) ([]lib.ArtistDB, bool, error) {
	var sb strings.Builder

	sb.WriteString(`SELECT id, name FROM artists`)

	var (
		conditions []string
		args       []any
	)

	// ID in priority
	if params.ID != nil {
		conditions = append(conditions, "id = ?")
		args = append(args, *params.ID)
	} else if len(params.IDs) > 0 {
		placeholders := make([]string, len(params.IDs))
		for i, id := range params.IDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(
			conditions,
			"id IN ("+strings.Join(placeholders, ", ")+")",
		)
	} else {
		if params.Name != nil {
			conditions = append(conditions, "name = ?")
			args = append(args, *params.Name)
		}
	}

	if len(conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	// sort
	if params.Random {
		sb.WriteString(" ORDER BY RANDOM() ")
	} else {
		sb.WriteString(" ORDER BY id ASC ")
	}

	// limit / offset
	if params.ID != nil {
		sb.WriteString(" LIMIT 1")
	} else if params.Limit > 0 {
		sb.WriteString(" LIMIT ? ")
		args = append(args, params.Limit+1)
		if params.Page != nil && *params.Page > 0 {
			offset := (*params.Page - 1) * params.Limit
			sb.WriteString(" OFFSET ? ")
			args = append(args, offset)
		}
	}

	rows, err := db.Query(sb.String(), args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer rows.Close()

	var artists []lib.ArtistDB

	var countRows int
	for rows.Next() {
		var t lib.ArtistDB

		err := rows.Scan(&t.ID, &t.Name)
		if err != nil {
			return nil, false, err
		}

		artists = append(artists, t)

		countRows++
	}

	if params.Limit > 0 && len(artists) > params.Limit {
		artists = slices.Delete(artists, len(artists)-1, len(artists))
	}
	return artists, params.Limit > 0 && countRows > params.Limit, rows.Err()
}

var ErrAlbumNotFound = errors.New("album not found")

func GetAlbum(db DBTX, params lib.AlbumsGetParams) (*lib.AlbumDB, error) {
	albums, _, err := GetAlbums(db, params)
	if err != nil {
		return nil, err
	}
	if len(albums) != 1 {
		return nil, ErrAlbumNotFound
	}
	return &albums[0], nil
}

func GetAlbums(db DBTX, params lib.AlbumsGetParams) ([]lib.AlbumDB, bool, error) {
	var sb strings.Builder

	sb.WriteString(`
		SELECT al.id, ar.name, al.artist_id, al.title, al.year
		FROM albums al
		LEFT JOIN artists ar ON ar.id = al.artist_id
	`)

	var (
		conditions []string
		args       []any
	)

	// ID in priority
	if params.ID != nil {
		conditions = append(conditions, "al.id = ?")
		args = append(args, *params.ID)
	} else {
		if params.ArtistID != nil {
			conditions = append(conditions, "al.artist_id = ?")
			args = append(args, *params.ArtistID)
		}
		if params.Title != nil {
			conditions = append(conditions, "al.title = ?")
			args = append(args, *params.Title)
		}
	}

	if len(conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	// sort
	if params.Random {
		sb.WriteString(" ORDER BY RANDOM() ")
	} else {
		sb.WriteString(" ORDER BY al.year DESC, al.id ASC ")
	}

	// limit / offset
	if params.ID != nil {
		sb.WriteString(" LIMIT 1")
	} else if params.Limit > 0 {
		sb.WriteString(" LIMIT ? ")
		args = append(args, params.Limit+1)
		if params.Page != nil && *params.Page > 0 {
			offset := (*params.Page - 1) * params.Limit
			sb.WriteString(" OFFSET ? ")
			args = append(args, offset)
		}
	}

	rows, err := db.Query(sb.String(), args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer rows.Close()

	var albums []lib.AlbumDB

	var countRows int
	for rows.Next() {
		var t lib.AlbumDB

		err := rows.Scan(&t.ID, &t.Artist, &t.ArtistID, &t.Title, &t.Year)
		if err != nil {
			return nil, false, err
		}

		albums = append(albums, t)

		countRows++
	}

	if params.Limit > 0 && len(albums) > params.Limit {
		albums = slices.Delete(albums, len(albums)-1, len(albums))
	}
	return albums, params.Limit > 0 && countRows > params.Limit, rows.Err()
}
