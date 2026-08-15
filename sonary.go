package main

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sonary/internal/config"
	"sonary/internal/database"
	"sonary/internal/foobar2000"
	"sonary/internal/job"
	"sonary/internal/lib"
	"sonary/internal/websocket"
	"sonary/utils"
	"strconv"
	"syscall"
	"time"
)

type API struct {
	readDB  *sql.DB
	writeDB *sql.DB
	//Config  config.Config
	//Store lib.Store
	//Logger     Logger
	//Mailer     Mailer
	//Cache      Cache
}

func (api *API) GetTracks(w http.ResponseWriter, r *http.Request) {
	var params lib.TracksGetParams
	limit := 50
	q := r.URL.Query()
	limitStr := q.Get("limit")
	var err error
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	params.Limit = limit
	page, _ := utils.QueryInt(q, "page")
	if page > 0 {
		params.Page = utils.Ptr(page)
	}

	var mode lib.FetchTracksMode
	if err := mode.UnmarshalText([]byte(q.Get("mode"))); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch mode {
	case lib.FetchTracksModeRandom:
		params.Random = true
	case lib.FetchTracksModeFavorites:
		params.Like = utils.Ptr(true)
	case lib.FetchTracksModeNoalbum:
		params.NoAlbum = true
	}

	artistID, _ := utils.QueryInt(q, "artistId")
	if artistID > 0 {
		params.ArtistID = utils.Ptr(artistID)
	}

	tracks, hasNext, err := database.GetTracks(api.readDB, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	directoryIDs := make([]int, 0, len(tracks))
	seen := make(map[int]struct{})
	for _, track := range tracks {
		if _, ok := seen[track.DirectoryID]; ok {
			continue
		}
		seen[track.DirectoryID] = struct{}{}
		directoryIDs = append(directoryIDs, track.DirectoryID)
	}

	images, err := database.GetImages(api.readDB, lib.ImagesGetParams{
		DirectoryIDs: directoryIDs,
		Type:         utils.Ptr(lib.ImageTypeMainFront),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiTracks := make([]lib.APITrack, len(tracks))
	for i, track := range tracks {
		apiTrack := track.ToAPI()
		if img, ok := images[track.DirectoryID]; ok {
			if len(img) > 0 {
				apiTrack.Cover = lib.ImageURLs(&img[0], lib.ImageTypeMainFront)
			}
		}
		apiTracks[i] = apiTrack
	}

	apiTrackList := lib.APITrackList{
		APIStatus: lib.APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		Items:   apiTracks,
		HasNext: hasNext,
	}

	w.Header().Set("Content-Type", "application/json")
	if js, err := json.Marshal(apiTrackList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Write(js)
	}
}

// get single track
func (api *API) GetTrack(w http.ResponseWriter, r *http.Request) {
	idVal := r.PathValue("id")
	id, err := strconv.Atoi(idVal)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	track, err := database.GetTrack(api.readDB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	images, err := database.GetImages(api.readDB, lib.ImagesGetParams{
		DirectoryIDs: []int{track.DirectoryID},
		Type:         utils.Ptr(lib.ImageTypeMainFront),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiTrack := lib.APITrackSingle{
		APIStatus: lib.APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		APITrack: track.ToAPI(),
	}

	if img, ok := images[track.DirectoryID]; ok {
		if len(img) > 0 {
			apiTrack.Cover = lib.ImageURLs(&img[0], lib.ImageTypeMainFront)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiTrack)
}

// update single track
func (api *API) UpdateTrack(w http.ResponseWriter, r *http.Request) {
	idVal := r.PathValue("id")
	id, err := strconv.Atoi(idVal)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var t lib.APITrackUpdate
	err = json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = database.UpdateTrack(api.writeDB, id, lib.TrackUpdateParams{
		Like: utils.Ptr(t.Like),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	track, err := database.GetTrack(api.readDB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiTrack := lib.APITrackSingle{
		APIStatus: lib.APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		APITrack: track.ToAPI(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiTrack)
}

func (api *API) GetArtists(w http.ResponseWriter, r *http.Request) {
	var params lib.ArtistsGetParams
	limit := 50
	q := r.URL.Query()
	limitStr := q.Get("limit")
	var err error
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	params.Limit = limit
	page, _ := utils.QueryInt(q, "page")
	if page > 0 {
		params.Page = utils.Ptr(page)
	}

	var mode lib.FetchArtistsMode
	if err := mode.UnmarshalText([]byte(q.Get("mode"))); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch mode {
	case lib.FetchArtistsModeRandom:
		params.Random = true
	}

	artists, hasNext, err := database.GetArtists(api.readDB, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	artistIDs := make([]int, 0, len(artists))
	for _, artist := range artists {
		artistIDs = append(artistIDs, artist.ID)
	}

	images, err := database.GetImages(api.readDB, lib.ImagesGetParams{
		ArtistIDs: artistIDs,
		GroupBy:   lib.ImageGroupByArtist,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiArtists := make([]lib.APIArtist, len(artists))
	for i, artist := range artists {
		apiArtist := artist.ToAPI()
		apiArtist.Images = []lib.APIImage{}
		if imgArtist, ok := images[artist.ID]; ok {
			if len(imgArtist) > 0 {
				for _, img := range imgArtist {
					if img.Type == lib.ImageTypeArtistLogo {
						apiArtist.Logo = lib.ImageURLs(&img, img.Type)
					}
					apiArtist.Images = append(apiArtist.Images, lib.APIImage{
						URL:   lib.ThumbnailURL(&img, lib.DefaultThumbnailSize),
						Type:  img.Type.String(),
						Order: img.Order,
					})
				}
			}
		}
		apiArtists[i] = apiArtist
	}
	apiArtistList := lib.APIArtistList{
		APIStatus: lib.APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		Items:   apiArtists,
		HasNext: hasNext,
	}

	w.Header().Set("Content-Type", "application/json")
	if js, err := json.Marshal(apiArtistList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Write(js)
	}
}

// get single artist
func (api *API) GetArtist(w http.ResponseWriter, r *http.Request) {
	idVal := r.PathValue("id")
	id, err := strconv.Atoi(idVal)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	artist, err := database.GetArtist(api.readDB, lib.ArtistsGetParams{ID: utils.Ptr(id)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	relatedArtistsIDs, err := database.GetRelatedArtists(api.readDB, artist.ID, database.RelationBoth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiRelatedArtists := []lib.APIArtist{}
	if len(relatedArtistsIDs) > 0 {
		relatedArtists, _, err := database.GetArtists(api.readDB, lib.ArtistsGetParams{IDs: relatedArtistsIDs})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, a := range relatedArtists {
			apiRelatedArtists = append(apiRelatedArtists, a.ToAPI())
		}
	}

	images, err := database.GetImagesFlat(api.readDB, lib.ImagesGetParams{
		ArtistIDs: []int{id},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiArtist := lib.APIArtistSingle{
		APIStatus: lib.APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		APIArtist:      artist.ToAPI(),
		RelatedArtists: apiRelatedArtists,
	}

	apiArtist.Images = []lib.APIImage{}
	for _, img := range images {
		if img.Type == lib.ImageTypeArtistLogo {
			apiArtist.Logo = lib.ImageURLs(&img, img.Type)
		}
		apiArtist.Images = append(apiArtist.Images, lib.APIImage{
			URL:   lib.ThumbnailURL(&img, lib.DefaultThumbnailSize),
			Type:  img.Type.String(),
			Order: img.Order,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiArtist)
}

func (api *API) GetAlbums(w http.ResponseWriter, r *http.Request) {
	var params lib.AlbumsGetParams
	limit := 50
	q := r.URL.Query()
	limitStr := q.Get("limit")
	var err error
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	params.Limit = limit
	page, _ := utils.QueryInt(q, "page")
	if page > 0 {
		params.Page = utils.Ptr(page)
	}

	var mode lib.FetchAlbumsMode
	if err := mode.UnmarshalText([]byte(q.Get("mode"))); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch mode {
	case lib.FetchAlbumsModeRandom:
		params.Random = true
	}

	artistID, _ := utils.QueryInt(q, "artistId")
	if artistID > 0 {
		params.ArtistID = utils.Ptr(artistID)
	}

	albums, hasNext, err := database.GetAlbums(api.readDB, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	albumIDs := make([]int, len(albums))
	artistIDs := make([]int, len(albums))
	seen := make(map[int]struct{})
	for _, album := range albums {
		albumIDs = append(albumIDs, album.ID)
		if _, ok := seen[album.ArtistID]; ok {
			continue
		}
		seen[album.ArtistID] = struct{}{}
		artistIDs = append(artistIDs, album.ArtistID)
	}

	imagesArtists, err := database.GetImages(api.readDB, lib.ImagesGetParams{
		ArtistIDs: artistIDs,
		Type:      utils.Ptr(lib.ImageTypeArtistLogo),
		GroupBy:   lib.ImageGroupByArtist,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	albumDirectories, err := database.GetAlbumDirectories(api.readDB, albumIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	directoryIDs := []int{}
	for _, dirs := range albumDirectories {
		directoryIDs = append(directoryIDs, dirs...)
	}

	images, err := database.GetImages(api.readDB, lib.ImagesGetParams{
		DirectoryIDs: directoryIDs,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiAlbums := make([]lib.APIAlbum, len(albums))
	for i, album := range albums {
		apiAlbum := album.ToAPI()
		if imgArtist, ok := imagesArtists[album.ArtistID]; ok {
			if len(imgArtist) > 0 {
				for _, img := range imgArtist {
					if img.Type == lib.ImageTypeArtistLogo {
						apiAlbum.ArtistLogo = lib.ImageURLs(&img, img.Type)
					}
				}
			}
		}
		if dirs, ok := albumDirectories[album.ID]; ok {
			for _, dirID := range dirs {
				if dirImg, ok := images[dirID]; ok {
					for _, img := range dirImg {
						if img.Type == lib.ImageTypeMainFront {
							apiAlbum.Cover = lib.ImageURLs(&img, img.Type)
						}
						apiAlbum.Images = append(apiAlbum.Images, lib.APIImage{
							URL:   lib.ThumbnailURL(&img, lib.DefaultThumbnailSize),
							Type:  img.Type.String(),
							Order: img.Order,
						})
					}
				}
			}
		}
		apiAlbums[i] = apiAlbum
	}
	apiAlbumList := lib.APIAlbumList{
		APIStatus: lib.APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		Items:   apiAlbums,
		HasNext: hasNext,
	}

	w.Header().Set("Content-Type", "application/json")
	if js, err := json.Marshal(apiAlbumList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Write(js)
	}
}

// get single album
func (api *API) GetAlbum(w http.ResponseWriter, r *http.Request) {
	idVal := r.PathValue("id")
	id, err := strconv.Atoi(idVal)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	album, err := database.GetAlbum(api.readDB, lib.AlbumsGetParams{ID: utils.Ptr(id)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tracks, _, err := database.GetTracks(api.readDB, lib.TracksGetParams{
		AlbumID: utils.Ptr(album.ID),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	directories, err := database.GetAlbumDirectories(api.readDB, []int{album.ID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	imagesArtist, err := database.GetImagesFlat(api.readDB, lib.ImagesGetParams{
		ArtistIDs: []int{album.ArtistID},
		Type:      utils.Ptr(lib.ImageTypeArtistLogo),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	images := []lib.Image{}
	if dirIDs, ok := directories[album.ID]; ok {
		images, err = database.GetImagesFlat(api.readDB, lib.ImagesGetParams{
			DirectoryIDs: dirIDs,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	apiTracks := make([]lib.APITrack, len(tracks))
	for i, track := range tracks {
		apiTracks[i] = track.ToAPI()
	}

	apiAlbum := lib.APIAlbumSingle{
		APIStatus: lib.APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		APIAlbum: album.ToAPI(),
		Tracks:   apiTracks,
	}

	if len(imagesArtist) > 0 {
		apiAlbum.ArtistLogo = lib.ImageURLs(&imagesArtist[0], lib.ImageTypeMainFront)
	}

	apiAlbum.Images = []lib.APIImage{}
	for _, img := range images {
		if img.Type == lib.ImageTypeMainFront {
			apiAlbum.Cover = lib.ImageURLs(&img, img.Type)
		}
		apiAlbum.Images = append(apiAlbum.Images, lib.APIImage{
			URL:   lib.ThumbnailURL(&img, lib.DefaultThumbnailSize),
			Type:  img.Type.String(),
			Order: img.Order,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiAlbum)
}

func (api *API) StartConvert(w http.ResponseWriter, r *http.Request) {
	var conv lib.APITrackConvertPost
	err := json.NewDecoder(r.Body).Decode(&conv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(conv.TrackIDs) == 1 && conv.Format == "mp3" {
		track, err := database.GetTrack(api.readDB, conv.TrackIDs[0])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if track.FileType == "MP3" {
			file, err := os.Open(track.Path)
			if err != nil {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			defer file.Close()

			w.Header().Set("Content-Type", "audio/mpeg")
			rawFilename := fmt.Sprintf("%s - %s.%s", track.Artist, track.Title, conv.Format)
			cleanFilename := utils.SanitizeFilename(rawFilename)
			utf8Filename := url.PathEscape(cleanFilename)
			disposition := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, cleanFilename, utf8Filename)
			w.Header().Set("Content-Disposition", disposition)
			w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")

			io.Copy(w, file)
			return
		}
	}

	// create job to convert tracks
	jobId, err := job.Enqueue(api.writeDB, sql.NullInt64{Valid: false},
		job.TaskConvertTracks, job.JobConvertTracks{
			UserID:   conv.UserID,
			TracksID: conv.TrackIDs,
			JobConvertParams: job.JobConvertParams{
				Format:        conv.Format,
				Mode:          conv.Mode,
				Quality:       conv.Quality,
				IncludePregap: conv.IncludePregap,
			},
		})
	if err != nil {
		log.Fatalf("Add job error: %v", err)
		http.Error(w, "Failed to enqueue job", http.StatusInternalServerError)
		return
	}

	apiConvert := lib.APIConvertTracks{
		APIStatus: lib.APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		JobID: jobId,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(apiConvert)
}

// download converted track or tracks
func (api *API) ConvertDownload(w http.ResponseWriter, r *http.Request) {
	idVal := r.PathValue("jobId")
	jobId, err := strconv.Atoi(idVal)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// find child jobs
	trackJobs, err := job.GetJobs(api.readDB, job.JobFilter{
		ParentID: utils.Ptr(jobId),
	}, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ct := lib.GetConvertContext()
	isMultiple := len(trackJobs) > 1
	zipWriter := new(zip.Writer)

	if isMultiple {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="tracks.zip"`)
		w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")

		zipWriter = zip.NewWriter(w)

		defer func() {
			zipWriter.Close()
		}()
	}

	for _, j := range trackJobs {
		var jobPayload job.JobConvertTrack
		err := json.Unmarshal(j.Payload, &jobPayload)
		if err != nil {
			http.Error(w, fmt.Sprintf("Job %d payload unmarshal error", j.ID), http.StatusInternalServerError)
			return
		}

		track, err := database.GetTrack(api.readDB, jobPayload.TrackID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if j.Status != job.StatusCompleted {
			http.Error(w, fmt.Sprintf("Job %d is not completed", j.ID), http.StatusInternalServerError)
			return
		}
		pathVal, ok := ct.Jobs.Load(j.ID)
		if !ok {
			http.Error(w, fmt.Sprintf("Job %d not found or expired", j.ID), http.StatusNotFound)
			return
		}
		filePath := pathVal.(string)
		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to open temp file: %v", err), http.StatusNotFound)
			return
		}

		if isMultiple {
			// multiple tracks processed - create archive
			rawFilename := fmt.Sprintf("%s - %s.%s", track.Artist, track.Title, jobPayload.Format)
			filenameInZip := utils.SanitizeFilename(rawFilename)
			header := &zip.FileHeader{
				Name:   filenameInZip,
				Method: zip.Deflate, // standard ZIP compression
			}
			// use UTF-8 for filename
			header.Flags |= 0x800

			writerInZip, err := zipWriter.CreateHeader(header)
			if err != nil {
				file.Close()
				http.Error(w, fmt.Sprintf("Failed to create zip entry header: %v", err), http.StatusInternalServerError)
				return
			}

			_, err = io.Copy(writerInZip, file)
			if err != nil {
				file.Close()
				http.Error(w, fmt.Sprintf("Failed to copy file to zip: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			// single track processed
			w.Header().Set("Content-Type", "audio/mpeg")
			rawFilename := fmt.Sprintf("%s - %s.%s", track.Artist, track.Title, jobPayload.Format)
			cleanFilename := utils.SanitizeFilename(rawFilename)
			utf8Filename := url.PathEscape(cleanFilename)
			disposition := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, cleanFilename, utf8Filename)
			w.Header().Set("Content-Disposition", disposition)
			w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
			io.Copy(w, file)
		}

		file.Close()
		os.Remove(filePath)
		ct.Jobs.Delete(j.ID)
	}
}

func (api *API) PlayInFoobar(w http.ResponseWriter, r *http.Request) {
	var play lib.APITrackPlayPost
	err := json.NewDecoder(r.Body).Decode(&play)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(play.TrackIDs) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	player := foobar2000.GetPlayer()

	if err := player.Play(play.TrackIDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Auth-Token")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func cacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}

func main() {
	cfg := config.GetConfig()

	readDB := database.Reader()
	defer readDB.Close()

	writeDB := database.Writer()
	defer writeDB.Close()

	// Create a background context that listens for system shutdown signals (Ctrl+C)
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()

	// Start the worker pool
	workerCount := cfg.WorkerCount
	log.Printf("Starting worker pool with %d concurrent workers...", workerCount)
	job.StartWorkerPool(workerCtx, writeDB, workerCount)

	if err := job.CancelJobs(writeDB); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}

	log.Println("Starting sync directories ...")
	_, err := job.Enqueue(writeDB, sql.NullInt64{Valid: false}, job.TaskSyncDirectories, nil)
	if err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}

	api := &API{
		readDB:  readDB,
		writeDB: writeDB,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/tracks", api.GetTracks)
	mux.HandleFunc("GET /api/v1/tracks/{id}", api.GetTrack)
	mux.HandleFunc("PUT /api/v1/tracks/{id}", api.UpdateTrack)
	mux.HandleFunc("GET /api/v1/artists", api.GetArtists)
	mux.HandleFunc("GET /api/v1/artists/{id}", api.GetArtist)
	mux.HandleFunc("GET /api/v1/albums", api.GetAlbums)
	mux.HandleFunc("GET /api/v1/albums/{id}", api.GetAlbum)
	mux.HandleFunc("POST /api/v1/convert/start", api.StartConvert)
	mux.HandleFunc("POST /api/v1/convert/download/{jobId}", api.ConvertDownload)
	mux.HandleFunc("POST /api/v1/play", api.PlayInFoobar)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("internal/templates/index.html"))
		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	})

	// serve files from the "./static" directory at the "/static/" URL path
	fileServer := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	// serve generated image thumbnails
	imageServer := http.FileServer(http.Dir(cfg.CacheDir))

	mux.Handle(
		"/api/v1/images/",
		cacheMiddleware(
			http.StripPrefix("/api/v1/images/", imageServer),
		),
	)

	// WebSocket
	hub := websocket.GetHub()
	mux.HandleFunc("/ws", websocket.WsEndpoint)
	go hub.Run()

	var handler http.Handler
	if cfg.AppEnv == "dev" {
		handler = corsMiddleware(mux)
	} else {
		handler = mux
	}

	srv := &http.Server{
		Addr:    cfg.Host,
		Handler: handler,
	}

	// Separate Goroutine to Handle the Shutdown Signal
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stopSignal // Execution stops here inside this goroutine until Ctrl+C
		log.Println("Shutdown signal received! Winding down...")

		// First, stop background workers from taking new jobs
		cancelWorkers()

		// Next, tell the web server to stop accepting new requests,
		// but give active requests 5 seconds to finish processing.
		shutdownCtx, cancelServer := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelServer()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server Shutdown Error: %v", err)
		}
	}()

	// Start the Web Server (This blocks the main thread)
	log.Println("Starting web server")
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP server failed to start: %v", err)
	}

	// Active Post-Shutdown Cleanup
	// This code runs AFTER the server is completely shut down and workers are canceled.
	log.Println("Performing final system cleanups...")
	time.Sleep(1 * time.Second) // Small buffer to let everything settle

	log.Println("Application stopped cleanly.")
}
