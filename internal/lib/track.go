// Package lib
package lib

import (
	"time"
)

type TrackOrDirectory interface {
	Type() string
}

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

func (m Artist) Type() string {
	return "artist"
}

func (m Album) Type() string {
	return "album"
}

func (m Track) Type() string {
	return "track"
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
	Title    string
	Year     int
}

type DirScan struct {
	Mtime    int64
	LastScan int64
}
