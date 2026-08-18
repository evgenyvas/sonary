package track

import (
	"fmt"
	"image"
	"image/color"
	stdDraw "image/draw"
	"image/jpeg"
	_ "image/png"
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

func isArtistLogo(name string) bool {
	n := strings.ToLower(name)

	if !strings.HasPrefix(n, "logo") {
		return false
	}

	switch filepath.Ext(n) {
	case ".jpg", ".jpeg", ".png":
		return true
	}

	return false
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
						return s.checkImage(entry, relImgPath, true)
					})
				if err != nil {
					log.Printf("Error walking path: '%s': %v\n", s.Path, err)
					return err
				}
			}
			continue
		}
		if err := s.checkImage(entry, entry.Name(), false); err != nil {
			return err
		}
	}
	//fmt.Printf("%#v\n", s.Images)
	return s.syncImages()
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
		s.Images = append(s.Images, lib.Image{
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

	// artist logo
	if isArtistLogo(n) {
		return lib.ImageTypeArtistLogo
	}

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

func (s *DirectoryScanner) syncImages() error {
	imagesByDirectory, err := database.GetImages(s.DB, lib.ImagesGetParams{
		DirectoryIDs: []int{s.Dir.ID},
	})
	if err != nil {
		return err
	}

	oldImages := imagesByDirectory[s.Dir.ID]

	oldByPath := make(map[string]lib.Image, len(oldImages))
	for _, img := range oldImages {
		oldByPath[img.Path] = img
	}

	newByPath := make(map[string]lib.Image, len(s.Images))
	for _, img := range s.Images {
		newByPath[img.Path] = img
	}

	// images that need to be removed from DB and whose thumbnails
	// need to be removed
	var imagesToDelete []lib.Image

	for _, oldImg := range oldImages {
		newImg, exists := newByPath[oldImg.Path]

		// image was deleted from filesystem
		if !exists {
			imagesToDelete = append(imagesToDelete, oldImg)
			continue
		}

		// image was changed
		if oldImg.Mtime != newImg.Mtime || oldImg.Size != newImg.Size {
			imagesToDelete = append(imagesToDelete, oldImg)
		}
	}

	// images that need to be inserted into DB and whose thumbnails
	// need to be generated
	var imagesToSave []lib.Image

	for _, img := range s.Images {
		oldImg, exists := oldByPath[img.Path]

		// new image
		if !exists {
			imagesToSave = append(imagesToSave, img)
			continue
		}

		// changed image
		if oldImg.Mtime != img.Mtime || oldImg.Size != img.Size {
			imagesToSave = append(imagesToSave, img)
		}
	}

	// nothing changed
	if len(imagesToDelete) == 0 && len(imagesToSave) == 0 {
		return nil
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// if there are artist logos, make sure ArtistID is known
	hasArtistLogo := false

	for _, img := range imagesToSave {
		if img.Type == lib.ImageTypeArtistLogo {
			hasArtistLogo = true
			break
		}
	}

	if hasArtistLogo && s.ArtistID == nil {
		artistName := filepath.Base(filepath.Clean(s.Path))
		if artistName == "." || artistName == "" {
			return fmt.Errorf("invalid artist directory: %q", s.Path)
		}
		artistID, err := database.GetOrAddArtist(tx, artistName)
		if err != nil {
			return err
		}
		s.ArtistID = &artistID
	}

	// delete old/removed/changed images from DB
	for _, img := range imagesToDelete {
		if err := database.DeleteImage(tx, img.ID); err != nil {
			return err
		}
	}

	// insert new/changed images into DB
	for i := range imagesToSave {
		if imagesToSave[i].Type == lib.ImageTypeArtistLogo {
			imagesToSave[i].ArtistID = s.ArtistID
		}
		if _, err := database.SaveImage(tx, s.Dir.ID, &imagesToSave[i]); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// DB is now synchronized. Remove thumbnails belonging to
	// deleted/changed images
	for _, img := range imagesToDelete {
		if err := s.Thumbnails.Delete(&img); err != nil {
			log.Printf("Thumbnail delete error '%s/%s': %v", s.Path, img.Path, err)
		}
		if _, exists := newByPath[img.Path]; exists {
			log.Printf("Image changed: '%s/%s'\n", s.Path, img.Path)
		} else {
			log.Printf("Image deleted: '%s/%s'\n", s.Path, img.Path)
		}
	}

	// Generate thumbnails for new/changed images
	if len(imagesToSave) > 0 {
		if err := s.Thumbnails.Generate(imagesToSave); err != nil {
			log.Printf("Thumbnail error: %v", err)
		}
	}

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

const thumbnailJPEGQuality = 90

func (g *ThumbnailGenerator) Generate(images []lib.Image) error {
	for _, img := range images {
		for _, size := range lib.ThumbnailSizesFor(img.Type) {
			if err := g.generateOne(&img, size); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *ThumbnailGenerator) generateOne(img *lib.Image, targetSize int) error {
	dstPath := g.thumbnailPath(img, targetSize)

	if info, err := os.Stat(dstPath); err == nil {
		if info.ModTime().Unix() == img.Mtime {
			return nil
		}
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

	// image is already small enough — don't resize, just save it as JPEG
	if img.Height <= targetSize {
		return saveJPEG(dstPath, src, img.Mtime)
	}

	var targetWidth, targetHeightPx int

	aspectRatio := float64(srcHeight) / float64(srcWidth)

	if aspectRatio >= 1.3 {
		// very tall image - limit width
		targetWidth = targetSize
		targetHeightPx = srcHeight * targetWidth / srcWidth
	} else {
		// regular or wide image - limit height
		targetHeightPx = targetSize
		targetWidth = srcWidth * targetHeightPx / srcHeight
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeightPx))

	draw.CatmullRom.Scale(
		dst,
		dst.Bounds(),
		src,
		bounds,
		draw.Over,
		nil,
	)

	return saveJPEG(dstPath, dst, img.Mtime)
}

func saveJPEG(dstPath string, img image.Image, mtime int64) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	// create an opaque white background
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)

	stdDraw.Draw(
		dst,
		bounds,
		image.NewUniform(color.White),
		image.Point{},
		stdDraw.Src,
	)

	// draw the original image over the white background
	// this removes transparency correctly
	stdDraw.Draw(
		dst,
		bounds,
		img,
		bounds.Min,
		stdDraw.Over,
	)

	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}

	if err := jpeg.Encode(out, dst, &jpeg.Options{
		Quality: thumbnailJPEGQuality,
	}); err != nil {
		out.Close()
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	t := time.Unix(mtime, 0)
	return os.Chtimes(dstPath, t, t)
}

func (g *ThumbnailGenerator) thumbnailPath(img *lib.Image, size int) string {
	return filepath.Join(
		g.CacheDir,
		fmt.Sprintf("%d", size),
		lib.ThumbnailFilename(img, size),
	)
}

func (g *ThumbnailGenerator) Delete(img *lib.Image) error {
	for _, size := range lib.ThumbnailSizesFor(img.Type) {
		path := g.thumbnailPath(img, size)
		err := os.Remove(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
	}
	return nil
}
