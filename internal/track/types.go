package track

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

type ImageSizes map[int]string
