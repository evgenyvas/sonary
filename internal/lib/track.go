// Package lib
package lib

import (
	"time"
)

type Artist struct {
	ID   int
	Name string
}

type Album struct {
	ID       int
	ArtistID int
	Title    string
	Year     int
}

type Track struct {
	ID          int
	AlbumID     int
	Path        string
	FileType    string
	Title       string
	Artist      string
	AlbumArtist string
	Year        int
	Genre       string
	Album       string
	TrackNumber int
	Duration    time.Duration
	Lyrics      string
	IsCue       bool
	CueFile     string
	CueOffset   time.Duration
	IsLike      bool
}

type DirScan struct {
	Mtime    int64
	LastScan int64
}

type DirDB struct {
	ID       int
	Path     string
	Mtime    int64
	LastScan int64
}

type ArtistDB struct {
	ID   int
	Name string
}

type AlbumDB struct {
	ID       int
	ArtistID int
	Artist   string
	Title    string
	Year     int
}

type TrackDB struct {
	ID          int
	Path        string
	FileType    string
	Title       string
	Artist      string
	ArtistID    int
	AlbumArtist string
	Year        int
	Genre       string
	Album       string
	AlbumID     int
	TrackNumber int
	Duration    time.Duration
	Lyrics      string
	IsCue       bool
	CueFile     string
	CueOffset   time.Duration
	IsLike      bool
}

type TracksGetParams struct {
	ID       *int
	AlbumID  *int
	ArtistID *int
	Random   bool
	Limit    int
	Page     *int
	Like     *bool
	NoAlbum  bool
}

type TrackUpdateParams struct {
	Like *bool
}

type ArtistsGetParams struct {
	ID    *int
	Name  *string
	Limit int
	Page  *int
}

type AlbumsGetParams struct {
	ID       *int
	ArtistID *int
	Random   bool
	Title    *string
	Limit    int
	Page     *int
}
