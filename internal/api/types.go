package api

import (
	"fmt"
	db "sonary/internal/database"
	"sonary/internal/track"
)

type APIStatus struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type APITrack struct {
	ID             int              `json:"id"`
	FileType       string           `json:"type"`
	Title          string           `json:"title"`
	Artist         string           `json:"artist"`
	ArtistID       int              `json:"artist_id"`
	AlbumArtist    string           `json:"albumArtist"`
	Year           int              `json:"year"`
	Genre          string           `json:"genre"`
	Album          string           `json:"album"`
	AlbumID        int              `json:"album_id"`
	Cover          track.ImageSizes `json:"cover,omitempty"`
	TrackNumber    int              `json:"number"`
	Duration       int              `json:"duration"`
	HasPregap      bool             `json:"pregap"`
	PregapDuration int              `json:"pregap_duration"`
	Lyrics         string           `json:"lyrics"`
	IsLike         bool             `json:"like"`
}

type APITrackSingle struct {
	APIStatus
	APITrack
}

type APITrackList struct {
	APIStatus
	Items   []APITrack `json:"items"`
	HasNext bool       `json:"next"`
}

type APITrackUpdate struct {
	Like bool `json:"like"`
}

type APIProgress struct {
	Total     int `json:"total"`
	Processed int `json:"processed"`
}

func TrackToAPI(t *db.Track) APITrack {
	return APITrack{
		ID:             t.ID,
		FileType:       t.FileType,
		Title:          t.Title,
		Artist:         t.Artist,
		ArtistID:       t.ArtistID,
		AlbumArtist:    t.AlbumArtist,
		Year:           t.Year,
		Genre:          t.Genre,
		Album:          t.Album,
		AlbumID:        t.AlbumID,
		TrackNumber:    t.TrackNumber,
		Duration:       int(t.Duration.Seconds()),
		HasPregap:      t.HasPregap,
		PregapDuration: int(t.PregapDuration.Seconds()),
		Lyrics:         t.Lyrics,
		IsLike:         t.IsLike,
	}
}

type FetchTracksMode string

const (
	FetchTracksModeAll       FetchTracksMode = "ALL"
	FetchTracksModeRandom    FetchTracksMode = "RANDOM"
	FetchTracksModeFavorites FetchTracksMode = "FAVORITES"
	FetchTracksModeNoalbum   FetchTracksMode = "NOALBUM" // tracks which artist is not equals to album artist
)

func (m *FetchTracksMode) UnmarshalText(text []byte) error {
	switch FetchTracksMode(text) {
	case FetchTracksModeAll,
		FetchTracksModeRandom,
		FetchTracksModeFavorites,
		FetchTracksModeNoalbum:
		*m = FetchTracksMode(text)
		return nil
	default:
		return fmt.Errorf("invalid mode: %q", text)
	}
}

type FetchArtistsMode string

const (
	FetchArtistsModeAll    FetchArtistsMode = "ALL"
	FetchArtistsModeRandom FetchArtistsMode = "RANDOM"
)

func (m *FetchArtistsMode) UnmarshalText(text []byte) error {
	switch FetchArtistsMode(text) {
	case FetchArtistsModeAll,
		FetchArtistsModeRandom:
		*m = FetchArtistsMode(text)
		return nil
	default:
		return fmt.Errorf("invalid mode: %q", text)
	}
}

type APIArtist struct {
	ID     int              `json:"id"`
	Name   string           `json:"name"`
	Logo   track.ImageSizes `json:"logo,omitempty"`
	Images []APIImage       `json:"images"`
}

type APIArtistSingle struct {
	APIStatus
	APIArtist
	RelatedArtists []APIArtist `json:"related"`
}

type APIArtistList struct {
	APIStatus
	Items   []APIArtist `json:"items"`
	HasNext bool        `json:"next"`
}

func ArtistToAPI(t *db.Artist) APIArtist {
	return APIArtist{
		ID:   t.ID,
		Name: t.Name,
	}
}

type APIImage struct {
	URL   string `json:"url"`
	Type  string `json:"type"`
	Order int    `json:"order"`
}

type APIAlbum struct {
	ID         int              `json:"id"`
	Artist     string           `json:"artist"`
	ArtistID   int              `json:"artist_id"`
	ArtistLogo track.ImageSizes `json:"artist_logo,omitempty"`
	Title      string           `json:"title"`
	Year       int              `json:"year"`
	Cover      track.ImageSizes `json:"cover,omitempty"`
	Images     []APIImage       `json:"images"`
}

type APIAlbumSingle struct {
	APIStatus
	APIAlbum
	Tracks []APITrack `json:"tracks"`
}

type APIAlbumList struct {
	APIStatus
	Items   []APIAlbum `json:"items"`
	HasNext bool       `json:"next"`
}

type APIConvertTracks struct {
	APIStatus
	JobID int64 `json:"job_id"`
}

func AlbumToAPI(t *db.Album) APIAlbum {
	return APIAlbum{
		ID:       t.ID,
		Artist:   t.Artist,
		ArtistID: t.ArtistID,
		Title:    t.Title,
		Year:     t.Year,
	}
}

type FetchAlbumsMode string

const (
	FetchAlbumsModeAll    FetchAlbumsMode = "ALL"
	FetchAlbumsModeRandom FetchAlbumsMode = "RANDOM"
)

func (m *FetchAlbumsMode) UnmarshalText(text []byte) error {
	switch FetchAlbumsMode(text) {
	case FetchAlbumsModeAll,
		FetchAlbumsModeRandom:
		*m = FetchAlbumsMode(text)
		return nil
	default:
		return fmt.Errorf("invalid mode: %q", text)
	}
}

type APITrackConvertPost struct {
	UserID        string `json:"userId"` // WebSocket userId
	TrackIDs      []int  `json:"track_ids"`
	Format        string `json:"format"`
	Mode          string `json:"mode"`
	Quality       string `json:"quality"`
	IncludePregap bool   `json:"include_pregap"`
}

type APITrackPlayPost struct {
	UserID   string `json:"userId"` // WebSocket userId
	TrackIDs []int  `json:"track_ids"`
}
