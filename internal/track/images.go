package track

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	stdDraw "image/draw"
	"image/jpeg"
	_ "image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	db "sonary/internal/database"
	"strconv"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"golang.org/x/image/draw"
)

const CoversDir = "Covers"

type ImageKeyword struct {
	Keyword string
	Type    ImageType
}

type ThumbnailGenerator struct {
	CacheDir string
}

type EmbeddedImage struct {
	Image db.Image
	Data  []byte
	Ext   string
}

// order matters - the first matching keyword wins
var imageKeywords = []ImageKeyword{
	{"front", ImageTypeFront},
	{"folder", ImageTypeFront},
	{"cover", ImageTypeFront},
	{"back", ImageTypeBack},
	{"disc", ImageTypeDisc},
	{"cd", ImageTypeDisc},
	{"dvd", ImageTypeDisc},
	{"bd", ImageTypeDisc},
	{"booklet", ImageTypeBooklet},
	{"inlay", ImageTypeInlay},
	{"inside", ImageTypeInside},
	{"digi", ImageTypeDigipack},
	{"slip", ImageTypeSlipcase},
	{"sticker", ImageTypeSticker},
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
	if err := s.syncImages(); err != nil {
		return err
	}

	// embedded images are synced for all tracks in the directory
	// if MainFront exists, we don't extract new embedded images,
	// but old embedded images still need to be removed
	s.EmbeddedTrackIDs = append(s.EmbeddedTrackIDs, s.ScannedTrackIDs...)

	// check embedded
	if !s.MainFrontFound {
		if err := s.processEmbeddedImages(); err != nil {
			return err
		}
	}
	return s.syncEmbeddedImages()
}

func (s *DirectoryScanner) processEmbeddedImages() error {
	for _, entry := range s.Entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))

		switch ext {
		case ".mp3", ".flac", ".ogg", ".m4a", ".wav":
		default:
			continue
		}

		fullPath := filepath.Join(s.Path, entry.Name())

		pictures, err := extractEmbeddedImages(fullPath)
		if err != nil {
			log.Printf("Embedded image error '%s': %v", fullPath, err)
			continue
		}
		if len(pictures) == 0 {
			continue
		}

		// get track by path
		track, err := db.GetTrackByPath(s.DB, fullPath)
		if err != nil {
			return err
		}
		if track == nil {
			log.Printf("Embedded image: track not found '%s'\n", fullPath)
			continue
		}

		for _, embedded := range pictures {
			if embedded.Ext == "" {
				log.Printf("unsupported embedded image format: track_id=%d", track.ID)
				continue
			}
			img, err := s.buildEmbeddedImage(*track, embedded)
			if err != nil {
				log.Printf("Embedded image error: %v", err)
				continue
			}
			embedded.Image = *img
			s.EmbeddedImages = append(s.EmbeddedImages, embedded)
		}
	}
	return nil
}

func (s *DirectoryScanner) syncEmbeddedImages() error {
	trackIDs := s.EmbeddedTrackIDs
	if len(trackIDs) == 0 {
		return nil
	}

	oldImages, err := db.GetImagesFlat(s.DB, db.ImagesGetParams{
		TrackIDs: trackIDs,
	})
	if err != nil {
		return err
	}

	oldByTrack := make(map[int]db.Image)

	for _, img := range oldImages {
		if img.TrackID == nil {
			continue
		}
		oldByTrack[*img.TrackID] = img
	}

	newByTrack := make(map[int]db.Image)

	for _, img := range s.EmbeddedImages {
		if img.Image.TrackID == nil {
			continue
		}
		newByTrack[*img.Image.TrackID] = img.Image
	}

	var imagesToDelete []db.Image
	var imagesToSave []EmbeddedImage

	// old images
	for _, oldImg := range oldImages {
		if oldImg.TrackID == nil {
			continue
		}

		trackID := *oldImg.TrackID
		newImg, exists := newByTrack[trackID]

		// embedded picture is not more exists
		if !exists {
			imagesToDelete = append(imagesToDelete, oldImg)
			continue
		}

		// embedded picture is changed
		if oldImg.Mtime != newImg.Mtime ||
			oldImg.Size != newImg.Size ||
			oldImg.Format != newImg.Format {
			imagesToDelete = append(imagesToDelete, oldImg)
		}
	}

	// new / changed
	for _, newImg := range s.EmbeddedImages {
		if newImg.Image.TrackID == nil {
			continue
		}

		trackID := *newImg.Image.TrackID

		oldImg, exists := oldByTrack[trackID]

		if !exists {
			imagesToSave = append(imagesToSave, newImg)
			continue
		}

		if oldImg.Mtime != newImg.Image.Mtime ||
			oldImg.Size != newImg.Image.Size ||
			oldImg.Format != newImg.Image.Format {
			imagesToSave = append(imagesToSave, newImg)
		}
	}

	if len(imagesToDelete) == 0 && len(imagesToSave) == 0 {
		return nil
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, img := range imagesToDelete {
		if err := db.DeleteImage(tx, img.ID); err != nil {
			return err
		}
	}

	for i := range imagesToSave {
		if err := db.SaveImage(tx, &imagesToSave[i].Image); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// delete old files
	for _, img := range imagesToDelete {
		if err := os.Remove(img.FullPath); err != nil && !os.IsNotExist(err) {
			log.Printf("Embedded image delete error '%s': %v", img.FullPath, err)
		}
	}

	// save new embedded images
	for _, img := range imagesToSave {
		if err := s.writeEmbeddedImage(&img); err != nil {
			log.Printf("Embedded image write error '%s': %v", img.Image.FullPath, err)
		}
	}

	if len(imagesToSave) > 0 {
		imgs := make([]db.Image, 0, len(imagesToSave))
		for _, img := range imagesToSave {
			imgs = append(imgs, img.Image)
		}
		if err := s.Thumbnails.Generate(imgs); err != nil {
			log.Printf("Embedded thumbnail error: %v", err)
		}
	}

	return nil
}

func (s *DirectoryScanner) buildEmbeddedImage(
	track db.Track,
	embedded EmbeddedImage,
) (*db.Image, error) {
	if embedded.Ext == "" {
		return nil, fmt.Errorf("embedded image has no extension: track_id=%d", track.ID)
	}

	audioInfo, err := os.Stat(track.Path)
	if err != nil {
		return nil, err
	}

	mtime := audioInfo.ModTime().Unix()

	path := s.embeddedImagePath(track.ID, embedded.Ext)

	cfg, format, err := image.DecodeConfig(bytes.NewReader(embedded.Data))
	if err != nil {
		return nil, err
	}

	return &db.Image{
		TrackID:     &track.ID,
		DirectoryID: nil,
		Path:        filepath.Join("embedded", filepath.Base(path)),
		FullPath:    path,
		Type:        int(ImageTypeMainFront),
		Format:      format,
		Order:       0,
		Width:       cfg.Width,
		Height:      cfg.Height,
		Size:        int64(len(embedded.Data)),
		Mtime:       mtime,
	}, nil
}

func (s *DirectoryScanner) embeddedImagePath(trackID int, ext string) string {
	return filepath.Join(
		s.Thumbnails.CacheDir,
		"embedded",
		fmt.Sprintf("track-%d.%s", trackID, ext),
	)
}

func (s *DirectoryScanner) writeEmbeddedImage(img *EmbeddedImage) error {
	if err := os.MkdirAll(filepath.Dir(img.Image.FullPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(img.Image.FullPath, img.Data, 0644); err != nil {
		return err
	}
	mtime := time.Unix(img.Image.Mtime, 0)
	return os.Chtimes(img.Image.FullPath, mtime, mtime)
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
		s.Images = append(s.Images, db.Image{
			DirectoryID: &s.Dir.ID,
			Path:        relImgPath,
			FullPath:    fullPath,
			Type:        int(s.detectImageType(entry.Name(), inCovers)),
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

func (s *DirectoryScanner) detectImageType(name string, inCovers bool) ImageType {
	n := strings.ToLower(name)

	// artist logo
	if isArtistLogo(n) {
		return ImageTypeArtistLogo
	}

	for _, t := range imageKeywords {
		if strings.Contains(n, t.Keyword) {
			// Front.jpg next to the music is the album's main image
			if !inCovers && t.Type == ImageTypeFront && !s.MainFrontFound {
				s.MainFrontFound = true
				return ImageTypeMainFront
			}
			return t.Type
		}
	}
	return ImageTypeOther
}

func (s *DirectoryScanner) syncImages() error {
	imagesByDirectory, err := db.GetImages(s.DB, db.ImagesGetParams{
		DirectoryIDs: []int{s.Dir.ID},
	})
	if err != nil {
		return err
	}

	oldImages := imagesByDirectory[s.Dir.ID]

	oldByPath := make(map[string]db.Image, len(oldImages))
	for _, img := range oldImages {
		oldByPath[img.Path] = img
	}

	newByPath := make(map[string]db.Image, len(s.Images))
	for _, img := range s.Images {
		newByPath[img.Path] = img
	}

	// images that need to be removed from DB and whose thumbnails
	// need to be removed
	var imagesToDelete []db.Image

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
	var imagesToSave []db.Image

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
		if img.Type == int(ImageTypeArtistLogo) {
			hasArtistLogo = true
			break
		}
	}

	if hasArtistLogo && s.ArtistID == nil {
		artistName := filepath.Base(filepath.Clean(s.Path))
		if artistName == "." || artistName == "" {
			return fmt.Errorf("invalid artist directory: %q", s.Path)
		}
		artistID, err := db.GetOrAddArtist(tx, artistName)
		if err != nil {
			return err
		}
		s.ArtistID = &artistID
	}

	// delete old/removed/changed images from DB
	for _, img := range imagesToDelete {
		if err := db.DeleteImage(tx, img.ID); err != nil {
			return err
		}
	}

	// insert new/changed images into DB
	for i := range imagesToSave {
		if imagesToSave[i].Type == int(ImageTypeArtistLogo) {
			imagesToSave[i].ArtistID = s.ArtistID
		}
		if err := db.SaveImage(tx, &imagesToSave[i]); err != nil {
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

func GetImageConfig(path string) (ImageConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return ImageConfig{}, err
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return ImageConfig{}, err
	}

	return ImageConfig{
		Width:  cfg.Width,
		Height: cfg.Height,
		Format: format,
	}, nil
}

const thumbnailJPEGQuality = 90

func (g *ThumbnailGenerator) Generate(images []db.Image) error {
	for _, img := range images {
		for _, size := range ThumbnailSizesFor(ImageType(img.Type)) {
			if err := g.generateOne(&img, size); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *ThumbnailGenerator) generateOne(img *db.Image, targetSize int) error {
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

func (g *ThumbnailGenerator) thumbnailPath(img *db.Image, size int) string {
	return filepath.Join(
		g.CacheDir,
		fmt.Sprintf("%d", size),
		ThumbnailFilename(img),
	)
}

func (g *ThumbnailGenerator) Delete(img *db.Image) error {
	for _, size := range ThumbnailSizesFor(ImageType(img.Type)) {
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

func extractEmbeddedImages(path string) ([]EmbeddedImage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	meta, err := tag.ReadFrom(f)
	if err != nil {
		if errors.Is(err, tag.ErrNoTagsFound) {
			return nil, nil
		}
		return nil, err
	}

	picture := meta.Picture()
	if picture == nil || len(picture.Data) == 0 {
		return nil, nil
	}

	return []EmbeddedImage{
		{
			Data: picture.Data,
			Ext:  embeddedImageExt(picture),
		},
	}, nil
}

func embeddedImageExt(picture *tag.Picture) string {
	ext := strings.TrimPrefix(strings.ToLower(picture.Ext), ".")
	switch ext {
	case "jpg", "jpeg":
		return "jpg"
	case "png":
		return "png"
	}
	switch strings.ToLower(picture.MIMEType) {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	default:
		return ""
	}
}

func ThumbnailFilename(img *db.Image) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(img.Path))

	if img.DirectoryID == nil {
		return fmt.Sprintf(
			"embedded_%02d_%016x.jpg",
			img.Type,
			h.Sum64(),
		)
	}

	return fmt.Sprintf(
		"%d_%02d_%016x.jpg",
		*img.DirectoryID,
		img.Type,
		h.Sum64(),
	)
}

func ThumbnailURL(img *db.Image, size int) string {
	return "/api/v1/images/" +
		strconv.Itoa(size) + "/" +
		ThumbnailFilename(img)
}

func ImageURLs(img *db.Image, tp ImageType) ImageSizes {
	sizes := ThumbnailSizesFor(tp)
	result := make(ImageSizes, len(sizes))
	for _, size := range sizes {
		result[size] = ThumbnailURL(img, size)
	}
	return result
}
