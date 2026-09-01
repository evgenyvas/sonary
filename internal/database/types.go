package database

import (
	"time"
)

type Directory struct {
	ID         int
	Path       string
	Mtime      int64
	LastScan   int64
	SideExists bool
	SideMtime  int64
}

type DirScan struct {
	Mtime      int64
	LastScan   int64
	SideExists bool
	SideMtime  int64
}

type Artist struct {
	ID   int
	Name string
}

type Album struct {
	ID           int
	ArtistID     int
	Artist       string
	Title        string
	Year         int
	DirectoryIDs []int
}

type Track struct {
	ID             int
	Path           string
	FileType       string
	Title          string
	Artist         string
	ArtistID       int
	AlbumArtist    string
	DirectoryID    int
	Year           int
	Genre          string
	Album          string
	AlbumID        int
	TrackNumber    int
	Duration       time.Duration
	HasPregap      bool
	PregapDuration time.Duration
	Lyrics         string
	IsCue          bool
	CueFile        string
	CueOffset      time.Duration
	IsLike         bool
}

type TracksGetParams struct {
	ID          *int
	AlbumID     *int
	ArtistID    *int
	DirectoryID *int
	Path        *string
	Random      bool
	Limit       int
	Page        *int
	Like        *bool
	NoAlbum     bool
	SearchQuery *string
}

type TrackUpdateParams struct {
	Like *bool
}

type ArtistsGetParams struct {
	ID          *int
	IDs         []int
	Name        *string
	Random      bool
	Limit       int
	Page        *int
	SearchQuery *string
}

type AlbumsGetParams struct {
	ID          *int
	ArtistID    *int
	Random      bool
	Title       *string
	Limit       int
	Page        *int
	SearchQuery *string
}

type ImageGroupBy int

const (
	ImageGroupByDirectory ImageGroupBy = iota
	ImageGroupByArtist
	ImageGroupByTrack
)

type ImagesGetParams struct {
	DirectoryIDs []int
	TrackIDs     []int
	ArtistIDs    []int
	Type         *int
	GroupBy      ImageGroupBy
}

type Image struct {
	ID          int
	DirectoryID *int
	TrackID     *int
	ArtistID    *int
	Path        string
	FullPath    string
	Type        int
	Format      string
	Order       int
	Width       int
	Height      int
	Size        int64
	Mtime       int64
	Embedded    bool
}
