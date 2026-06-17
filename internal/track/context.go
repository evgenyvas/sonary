package track

import (
	"sonary/internal/database"
	"sonary/internal/lib"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	instance *ImportContext
	once     sync.Once
)

type Progress struct {
	Total     int
	Processed atomic.Int64
}

type ImportContext struct {
	ArtistCache map[string]int
	AlbumCache  map[string]int
	Progress    Progress
}

func GetImportContext(total int) *ImportContext {
	once.Do(func() {
		instance = &ImportContext{
			ArtistCache: map[string]int{},
			AlbumCache:  map[string]int{},
			Progress: Progress{
				Total: total,
			},
		}
	})
	if total != 0 {
		instance.ArtistCache = map[string]int{}
		instance.AlbumCache = map[string]int{}
		instance.Progress.Total = 0
		instance.Progress.Processed.Store(0)
	}
	return instance
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
