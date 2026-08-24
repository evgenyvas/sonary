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
	ID             int
	AlbumID        int
	Path           string
	FileType       string
	Title          string
	Artist         string
	AlbumArtist    string
	Year           int
	Genre          string
	Album          string
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

type DirScan struct {
	Mtime      int64
	LastScan   int64
	SideExists bool
	SideMtime  int64
}

type DirDB struct {
	ID         int
	Path       string
	Mtime      int64
	LastScan   int64
	SideExists bool
	SideMtime  int64
}

type ArtistDB struct {
	ID   int
	Name string
}

type AlbumDB struct {
	ID           int
	ArtistID     int
	Artist       string
	Title        string
	Year         int
	DirectoryIDs []int
}

type TrackDB struct {
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

type ConvertParams struct {
	Format        string
	Mode          string
	Quality       string
	IncludePregap bool
}

func (params *ConvertParams) ToString() string {
	pr := "pregap"
	if !params.IncludePregap {
		pr = "no" + pr
	}
	return params.Format + "_" + params.Mode + "_" + params.Quality + "_" + pr
}

const DefaultThumbnailSize = 640

type ImageType int

const (
	ImageTypeArtistLogo ImageType = iota
	ImageTypeMainFront
	ImageTypeFront
	ImageTypeBack
	ImageTypeDisc
	ImageTypeBooklet
	ImageTypeInlay
	ImageTypeInside
	ImageTypeDigipack
	ImageTypeSlipcase
	ImageTypeSticker
	ImageTypeOther
)

func (t ImageType) String() string {
	switch t {
	case ImageTypeArtistLogo:
		return "artist-logo"
	case ImageTypeMainFront:
		return "main-front"
	case ImageTypeFront:
		return "front"
	case ImageTypeBack:
		return "back"
	case ImageTypeDisc:
		return "disc"
	case ImageTypeBooklet:
		return "booklet"
	case ImageTypeInlay:
		return "inlay"
	case ImageTypeInside:
		return "inside"
	case ImageTypeDigipack:
		return "digipack"
	case ImageTypeSlipcase:
		return "slipcase"
	case ImageTypeOther:
		return "other"
	default:
		return "unknown"
	}
}

type Image struct {
	ID          int
	DirectoryID *int
	TrackID     *int
	ArtistID    *int
	Path        string
	FullPath    string
	Type        ImageType
	Format      string
	Order       int
	Width       int
	Height      int
	Size        int64
	Mtime       int64
	Embedded    bool
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
	Type         *ImageType
	GroupBy      ImageGroupBy
}

type ImageConfig struct {
	Width, Height int
	Format        string
}

var (
	defaultThumbnailSizes = []int{DefaultThumbnailSize}

	thumbnailSizes = map[ImageType][]int{
		ImageTypeArtistLogo: {
			160,
			320,
			DefaultThumbnailSize,
		},
		ImageTypeMainFront: {
			160,
			320,
			DefaultThumbnailSize,
		},
	}
)

func ThumbnailSizesFor(t ImageType) []int {
	if sizes, ok := thumbnailSizes[t]; ok {
		return sizes
	}
	return defaultThumbnailSizes
}

type ScanFileType int

const (
	ScanFileTypeSide ScanFileType = iota
	ScanFileTypeLogo
	ScanFileTypeAudio
)
