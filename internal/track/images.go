package track

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sonary/internal/database"
	"sonary/internal/lib"
	"strings"
	"time"

	"golang.org/x/image/draw"
)

const CoversDir = "Covers"

type ImageKeyword struct {
	Keyword string
	Type    lib.ImageType
}

type ThumbnailGenerator struct {
	CacheDir string
}

// order matters - the first matching keyword wins
var imageKeywords = []ImageKeyword{
	{"front", lib.ImageTypeFront},
	{"folder", lib.ImageTypeFront},
	{"cover", lib.ImageTypeFront},
	{"back", lib.ImageTypeBack},
	{"disc", lib.ImageTypeDisc},
	{"cd", lib.ImageTypeDisc},
	{"dvd", lib.ImageTypeDisc},
	{"bd", lib.ImageTypeDisc},
	{"booklet", lib.ImageTypeBooklet},
	{"inlay", lib.ImageTypeInlay},
	{"inside", lib.ImageTypeInside},
	{"digi", lib.ImageTypeDigipack},
	{"slip", lib.ImageTypeSlipcase},
}

func (s *DirectoryScanner) processImages() error {
	for _, entry := range s.Entries {
		if entry.IsDir() {
			if strings.EqualFold(entry.Name(), CoversDir) {
				// search booklet images recursively
				err := filepath.WalkDir(filepath.Join(s.Path, entry.Name()),
					func(path string, entry fs.DirEntry, err error) error {
						if err != nil {
							return err
						}
						if entry.IsDir() {
							return nil
						}
						relImgPath, err := filepath.Rel(s.Path, path)
						if err != nil {
							return err
						}
						if err := s.checkImage(entry, relImgPath, false); err != nil {
							return err
						}
						return nil
					})
				if err != nil {
					log.Printf("Error walking path: '%s': %v\n", s.Path, err)
					return err
				}
			}
			continue
		}
		if err := s.checkImage(entry, entry.Name(), true); err != nil {
			return err
		}
	}
	//fmt.Printf("%#v\n", s.Images)
	//if err := s.saveImages(); err != nil {
	//return err
	//}
	if err := s.Thumbnails.Generate(s.Images); err != nil {
		log.Printf("Thumbnail error: %v", err)
	}
	return nil
}

func (s *DirectoryScanner) checkImage(entry os.DirEntry, relImgPath string, inCovers bool) error {
	ext := strings.ToLower(filepath.Ext(entry.Name()))
	switch ext {
	case ".jpg", ".jpeg", ".png":
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fullPath := filepath.Join(s.Path, relImgPath)
		cfg, err := GetImageConfig(fullPath)
		if err != nil {
			return err
		}
		s.Images = append(s.Images, lib.DirectoryImage{
			DirectoryID: s.Dir.ID,
			Path:        relImgPath,
			FullPath:    fullPath,
			Type:        s.detectImageType(entry.Name(), inCovers),
			Format:      cfg.Format,
			Order:       s.ImageOrder,
			Width:       cfg.Width,
			Height:      cfg.Height,
			Size:        info.Size(),
			Mtime:       info.ModTime().Unix(),
		})
		s.ImageOrder++
	}
	return nil
}

func (s *DirectoryScanner) detectImageType(name string, inCovers bool) lib.ImageType {
	n := strings.ToLower(name)
	for _, t := range imageKeywords {
		if strings.Contains(n, t.Keyword) {
			// Front.jpg next to the music is the album's main image
			if !inCovers && t.Type == lib.ImageTypeFront && !s.MainFrontFound {
				s.MainFrontFound = true
				return lib.ImageTypeMainFront
			}
			return t.Type
		}
	}
	return lib.ImageTypeOther
}

func (s *DirectoryScanner) saveImages() error {
	if len(s.Images) == 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, img := range s.Images {
		_, err = database.SaveImage(tx, s.Dir.ID, &img)
		if err != nil {
			return err
		}
	}

	if err = database.UpdateDirectoryLastScan(tx, s.Dir.ID); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	log.Printf("Images saved successfully '%s'\n", s.Path)
	return nil
}

func GetImageConfig(path string) (lib.ImageConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return lib.ImageConfig{}, err
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return lib.ImageConfig{}, err
	}

	return lib.ImageConfig{
		Width:  cfg.Width,
		Height: cfg.Height,
		Format: format,
	}, nil
}

const defaultThumbnailHeight = 640
const thumbnailJPEGQuality = 90

var (
	defaultThumbnailHeights = []int{defaultThumbnailHeight}

	frontThumbnailHeights = []int{
		160,
		320,
		defaultThumbnailHeight,
	}

	thumbnailHeights = map[lib.ImageType][]int{
		lib.ImageTypeMainFront: frontThumbnailHeights,
		lib.ImageTypeFront:     frontThumbnailHeights,
	}
)

func thumbnailHeightsFor(t lib.ImageType) []int {
	if heights, ok := thumbnailHeights[t]; ok {
		return heights
	}
	return defaultThumbnailHeights
}

func (g *ThumbnailGenerator) Generate(images []lib.DirectoryImage) error {
	for _, img := range images {
		for _, height := range thumbnailHeightsFor(img.Type) {
			if err := g.generateOne(&img, height); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *ThumbnailGenerator) generateOne(img *lib.DirectoryImage, targetHeight int) error {
	dstPath := g.thumbnailPath(img, targetHeight)
	if info, err := os.Stat(dstPath); err == nil {
		if info.ModTime().Unix() == img.Mtime {
			return nil
		}
	}

	// if the original is already smaller than the required height just copy original
	if img.Height <= targetHeight {
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}
		return copyFile(img.FullPath, dstPath)
	}

	srcFile, err := os.Open(img.FullPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	src, _, err := image.Decode(srcFile)
	if err != nil {
		return err
	}

	bounds := src.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()

	targetWidth := srcWidth * targetHeight / srcHeight

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	draw.CatmullRom.Scale(
		dst,
		dst.Bounds(),
		src,
		bounds,
		draw.Over,
		nil,
	)

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}

	if err := jpeg.Encode(out, dst, &jpeg.Options{
		Quality: thumbnailJPEGQuality,
	}); err != nil {
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	t := time.Unix(img.Mtime, 0)
	return os.Chtimes(dstPath, t, t)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

func (g *ThumbnailGenerator) thumbnailPath(img *lib.DirectoryImage, height int) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(img.Path))

	filename := fmt.Sprintf(
		"%d_%03d_%02d_%016x.jpg",
		img.DirectoryID,
		img.Order,
		img.Type,
		h.Sum64(),
	)

	return filepath.Join(
		g.CacheDir,
		fmt.Sprintf("%d", height),
		filename,
	)
}
