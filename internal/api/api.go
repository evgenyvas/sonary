// Package api
package api

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	appContext "sonary/internal/context"
	db "sonary/internal/database"
	"sonary/internal/foobar2000"
	"sonary/internal/job"
	"sonary/internal/track"
	"sonary/utils"
	"strconv"
)

type API struct {
	ReadDB  *sql.DB
	WriteDB *sql.DB
	//Config  config.Config
	//Store lib.Store
	//Logger     Logger
	//Mailer     Mailer
	//Cache      Cache
}

func (api *API) GetTracks(w http.ResponseWriter, r *http.Request) {
	var params db.TracksGetParams
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

	var mode FetchTracksMode
	if err := mode.UnmarshalText([]byte(q.Get("mode"))); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch mode {
	case FetchTracksModeRandom:
		params.Random = true
	case FetchTracksModeFavorites:
		params.Like = utils.Ptr(true)
	case FetchTracksModeNoalbum:
		params.NoAlbum = true
	}

	artistID, _ := utils.QueryInt(q, "artistId")
	if artistID > 0 {
		params.ArtistID = utils.Ptr(artistID)
	}

	searchQuery, err := url.PathUnescape(utils.QueryString(q, "searchQuery"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if searchQuery != "" {
		params.SearchQuery = utils.Ptr(searchQuery)
	}

	tracks, hasNext, err := db.GetTracks(api.ReadDB, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	directoryIDs := make([]int, 0, len(tracks))
	trackIDs := make([]int, 0, len(tracks))
	seenDirectories := make(map[int]struct{})

	for _, t := range tracks {
		trackIDs = append(trackIDs, t.ID)
		if _, ok := seenDirectories[t.DirectoryID]; ok {
			continue
		}
		seenDirectories[t.DirectoryID] = struct{}{}
		directoryIDs = append(directoryIDs, t.DirectoryID)
	}

	directoryImages, err := db.GetImages(api.ReadDB, db.ImagesGetParams{
		DirectoryIDs: directoryIDs,
		Type:         utils.Ptr(int(track.ImageTypeMainFront)),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	embeddedImages, err := db.GetImages(api.ReadDB, db.ImagesGetParams{
		TrackIDs: trackIDs,
		Type:     utils.Ptr(int(track.ImageTypeMainFront)),
		GroupBy:  db.ImageGroupByTrack,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiTracks := make([]APITrack, len(tracks))
	for i, t := range tracks {
		apiTrack := TrackToAPI(&t)
		if img, ok := directoryImages[t.DirectoryID]; ok {
			if len(img) > 0 {
				apiTrack.Cover = track.ImageURLs(&img[0], track.ImageTypeMainFront)
			}
		}
		if apiTrack.Cover == nil {
			if imgs, ok := embeddedImages[t.ID]; ok {
				if len(imgs) > 0 {
					apiTrack.Cover = track.ImageURLs(&imgs[0], track.ImageTypeMainFront)
				}
			}
		}
		apiTracks[i] = apiTrack
	}

	apiTrackList := APITrackList{
		APIStatus: APIStatus{
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

	t, err := db.GetTrack(api.ReadDB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	directoryImages, err := db.GetImages(api.ReadDB, db.ImagesGetParams{
		DirectoryIDs: []int{t.DirectoryID},
		Type:         utils.Ptr(int(track.ImageTypeMainFront)),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiTrack := APITrackSingle{
		APIStatus: APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		APITrack: TrackToAPI(t),
	}

	if img, ok := directoryImages[t.DirectoryID]; ok {
		if len(img) > 0 {
			apiTrack.Cover = track.ImageURLs(&img[0], track.ImageTypeMainFront)
		}
	}
	if apiTrack.Cover == nil {
		embeddedImages, err := db.GetImages(api.ReadDB, db.ImagesGetParams{
			TrackIDs: []int{t.ID},
			Type:     utils.Ptr(int(track.ImageTypeMainFront)),
			GroupBy:  db.ImageGroupByTrack,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if img, ok := embeddedImages[t.ID]; ok {
			if len(img) > 0 {
				apiTrack.Cover = track.ImageURLs(&img[0], track.ImageTypeMainFront)
			}
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

	var upd APITrackUpdate
	err = json.NewDecoder(r.Body).Decode(&upd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = db.UpdateTrack(api.WriteDB, id, db.TrackUpdateParams{
		Like: utils.Ptr(upd.Like),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t, err := db.GetTrack(api.ReadDB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	directoryImages, err := db.GetImages(api.ReadDB, db.ImagesGetParams{
		DirectoryIDs: []int{t.DirectoryID},
		Type:         utils.Ptr(int(track.ImageTypeMainFront)),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiTrack := APITrackSingle{
		APIStatus: APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		APITrack: TrackToAPI(t),
	}

	if img, ok := directoryImages[t.DirectoryID]; ok {
		if len(img) > 0 {
			apiTrack.Cover = track.ImageURLs(&img[0], track.ImageTypeMainFront)
		}
	}
	if apiTrack.Cover == nil {
		embeddedImages, err := db.GetImages(api.ReadDB, db.ImagesGetParams{
			TrackIDs: []int{t.ID},
			Type:     utils.Ptr(int(track.ImageTypeMainFront)),
			GroupBy:  db.ImageGroupByTrack,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if img, ok := embeddedImages[t.ID]; ok {
			if len(img) > 0 {
				apiTrack.Cover = track.ImageURLs(&img[0], track.ImageTypeMainFront)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiTrack)
}

func (api *API) GetArtists(w http.ResponseWriter, r *http.Request) {
	var params db.ArtistsGetParams
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

	var mode FetchArtistsMode
	if err := mode.UnmarshalText([]byte(q.Get("mode"))); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch mode {
	case FetchArtistsModeRandom:
		params.Random = true
	}

	searchQuery, err := url.PathUnescape(utils.QueryString(q, "searchQuery"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if searchQuery != "" {
		params.SearchQuery = utils.Ptr(searchQuery)
	}

	artists, hasNext, err := db.GetArtists(api.ReadDB, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	artistIDs := make([]int, 0, len(artists))
	for _, artist := range artists {
		artistIDs = append(artistIDs, artist.ID)
	}

	images, err := db.GetImages(api.ReadDB, db.ImagesGetParams{
		ArtistIDs: artistIDs,
		GroupBy:   db.ImageGroupByArtist,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiArtists := make([]APIArtist, len(artists))
	for i, artist := range artists {
		apiArtist := ArtistToAPI(&artist)
		apiArtist.Images = []APIImage{}
		if imgArtist, ok := images[artist.ID]; ok {
			if len(imgArtist) > 0 {
				for _, img := range imgArtist {
					tp := track.ImageType(img.Type)
					if img.Type == int(track.ImageTypeArtistLogo) {
						apiArtist.Logo = track.ImageURLs(&img, tp)
					}
					apiArtist.Images = append(apiArtist.Images, APIImage{
						URL:   track.ThumbnailURL(&img, track.DefaultThumbnailSize),
						Type:  tp.String(),
						Order: img.Order,
					})
				}
			}
		}
		apiArtists[i] = apiArtist
	}
	apiArtistList := APIArtistList{
		APIStatus: APIStatus{
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

	artist, err := db.GetArtist(api.ReadDB, db.ArtistsGetParams{ID: utils.Ptr(id)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	relatedArtistsIDs, err := db.GetRelatedArtists(api.ReadDB, artist.ID, db.RelationBoth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiRelatedArtists := []APIArtist{}
	if len(relatedArtistsIDs) > 0 {
		relatedArtists, _, err := db.GetArtists(api.ReadDB, db.ArtistsGetParams{IDs: relatedArtistsIDs})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, a := range relatedArtists {
			apiRelatedArtists = append(apiRelatedArtists, ArtistToAPI(&a))
		}
	}

	images, err := db.GetImagesFlat(api.ReadDB, db.ImagesGetParams{
		ArtistIDs: []int{id},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiArtist := APIArtistSingle{
		APIStatus: APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		APIArtist:      ArtistToAPI(artist),
		RelatedArtists: apiRelatedArtists,
	}

	apiArtist.Images = []APIImage{}
	for _, img := range images {
		tp := track.ImageType(img.Type)
		if img.Type == int(track.ImageTypeArtistLogo) {
			apiArtist.Logo = track.ImageURLs(&img, tp)
		}
		apiArtist.Images = append(apiArtist.Images, APIImage{
			URL:   track.ThumbnailURL(&img, track.DefaultThumbnailSize),
			Type:  tp.String(),
			Order: img.Order,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiArtist)
}

func (api *API) GetAlbums(w http.ResponseWriter, r *http.Request) {
	var params db.AlbumsGetParams
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

	var mode FetchAlbumsMode
	if err := mode.UnmarshalText([]byte(q.Get("mode"))); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch mode {
	case FetchAlbumsModeRandom:
		params.Random = true
	}

	artistID, _ := utils.QueryInt(q, "artistId")
	if artistID > 0 {
		params.ArtistID = utils.Ptr(artistID)
	}

	searchQuery, err := url.PathUnescape(utils.QueryString(q, "searchQuery"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if searchQuery != "" {
		params.SearchQuery = utils.Ptr(searchQuery)
	}

	albums, hasNext, err := db.GetAlbums(api.ReadDB, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	albumIDs := make([]int, 0, len(albums))
	artistIDs := make([]int, 0, len(albums))
	seen := make(map[int]struct{})
	for _, album := range albums {
		albumIDs = append(albumIDs, album.ID)
		if _, ok := seen[album.ArtistID]; ok {
			continue
		}
		seen[album.ArtistID] = struct{}{}
		artistIDs = append(artistIDs, album.ArtistID)
	}

	imagesArtists, err := db.GetImages(api.ReadDB, db.ImagesGetParams{
		ArtistIDs: artistIDs,
		Type:      utils.Ptr(int(track.ImageTypeArtistLogo)),
		GroupBy:   db.ImageGroupByArtist,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	albumDirectories, err := db.GetAlbumDirectories(api.ReadDB, albumIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	directoryIDs := []int{}
	for _, dirs := range albumDirectories {
		directoryIDs = append(directoryIDs, dirs...)
	}

	directoryImages, err := db.GetImages(api.ReadDB, db.ImagesGetParams{
		DirectoryIDs: directoryIDs,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiAlbums := make([]APIAlbum, len(albums))
	for i, album := range albums {
		apiAlbum := AlbumToAPI(&album)
		if imgArtist, ok := imagesArtists[album.ArtistID]; ok {
			if len(imgArtist) > 0 {
				for _, img := range imgArtist {
					if img.Type == int(track.ImageTypeArtistLogo) {
						apiAlbum.ArtistLogo = track.ImageURLs(&img, track.ImageType(img.Type))
					}
				}
			}
		}
		if dirs, ok := albumDirectories[album.ID]; ok {
			for _, dirID := range dirs {
				if dirImg, ok := directoryImages[dirID]; ok {
					for _, img := range dirImg {
						tp := track.ImageType(img.Type)
						if img.Type == int(track.ImageTypeMainFront) {
							apiAlbum.Cover = track.ImageURLs(&img, tp)
						}
						apiAlbum.Images = append(apiAlbum.Images, APIImage{
							URL:   track.ThumbnailURL(&img, track.DefaultThumbnailSize),
							Type:  tp.String(),
							Order: img.Order,
						})
					}
				}
			}
		}
		if apiAlbum.Cover == nil {
			tracks, _, err := db.GetTracks(api.ReadDB, db.TracksGetParams{
				AlbumID: utils.Ptr(album.ID),
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			tracksIDs := []int{}
			for _, track := range tracks {
				tracksIDs = append(tracksIDs, track.ID)
			}
			embeddedImages, err := db.GetImagesFlat(api.ReadDB, db.ImagesGetParams{
				TrackIDs: tracksIDs,
				Type:     utils.Ptr(int(track.ImageTypeMainFront)),
				GroupBy:  db.ImageGroupByTrack,
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			for _, img := range embeddedImages {
				if img.Type == int(track.ImageTypeMainFront) {
					apiAlbum.Cover = track.ImageURLs(&img, track.ImageType(img.Type))
					break
				}
			}
		}
		apiAlbums[i] = apiAlbum
	}
	apiAlbumList := APIAlbumList{
		APIStatus: APIStatus{
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

	album, err := db.GetAlbum(api.ReadDB, db.AlbumsGetParams{ID: utils.Ptr(id)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tracks, _, err := db.GetTracks(api.ReadDB, db.TracksGetParams{
		AlbumID: utils.Ptr(album.ID),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tracksIDs := []int{}
	for _, t := range tracks {
		tracksIDs = append(tracksIDs, t.ID)
	}

	directories, err := db.GetAlbumDirectories(api.ReadDB, []int{album.ID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	imagesArtist, err := db.GetImagesFlat(api.ReadDB, db.ImagesGetParams{
		ArtistIDs: []int{album.ArtistID},
		Type:      utils.Ptr(int(track.ImageTypeArtistLogo)),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	directoryImages := []db.Image{}
	if dirIDs, ok := directories[album.ID]; ok {
		directoryImages, err = db.GetImagesFlat(api.ReadDB, db.ImagesGetParams{
			DirectoryIDs: dirIDs,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	embeddedImages, err := db.GetImagesFlat(api.ReadDB, db.ImagesGetParams{
		TrackIDs: tracksIDs,
		Type:     utils.Ptr(int(track.ImageTypeMainFront)),
		GroupBy:  db.ImageGroupByTrack,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiTracks := make([]APITrack, len(tracks))
	for i, t := range tracks {
		apiTrack := TrackToAPI(&t)
		for _, img := range directoryImages {
			if img.Type == int(track.ImageTypeMainFront) {
				apiTrack.Cover = track.ImageURLs(&img, track.ImageType(img.Type))
				break
			}
		}
		if apiTrack.Cover == nil {
			for _, img := range embeddedImages {
				if img.Type == int(track.ImageTypeMainFront) {
					apiTrack.Cover = track.ImageURLs(&img, track.ImageType(img.Type))
					break
				}
			}
		}
		apiTracks[i] = apiTrack
	}

	apiAlbum := APIAlbumSingle{
		APIStatus: APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		APIAlbum: AlbumToAPI(album),
		Tracks:   apiTracks,
	}

	if len(imagesArtist) > 0 {
		apiAlbum.ArtistLogo = track.ImageURLs(&imagesArtist[0], track.ImageTypeMainFront)
	}

	apiAlbum.Images = []APIImage{}
	for _, img := range directoryImages {
		tp := track.ImageType(img.Type)
		if img.Type == int(track.ImageTypeMainFront) {
			apiAlbum.Cover = track.ImageURLs(&img, tp)
		}
		apiAlbum.Images = append(apiAlbum.Images, APIImage{
			URL:   track.ThumbnailURL(&img, track.DefaultThumbnailSize),
			Type:  tp.String(),
			Order: img.Order,
		})
	}
	for _, img := range embeddedImages {
		tp := track.ImageType(img.Type)
		if img.Type == int(track.ImageTypeMainFront) {
			apiAlbum.Cover = track.ImageURLs(&img, tp)
		}
		apiAlbum.Images = append(apiAlbum.Images, APIImage{
			URL:   track.ThumbnailURL(&img, track.DefaultThumbnailSize),
			Type:  tp.String(),
			Order: img.Order,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiAlbum)
}

func (api *API) StartConvert(w http.ResponseWriter, r *http.Request) {
	var conv APITrackConvertPost
	err := json.NewDecoder(r.Body).Decode(&conv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(conv.TrackIDs) == 1 && conv.Format == "mp3" {
		track, err := db.GetTrack(api.ReadDB, conv.TrackIDs[0])
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
	jobId, err := job.Enqueue(api.WriteDB, sql.NullInt64{Valid: false},
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

	apiConvert := APIConvertTracks{
		APIStatus: APIStatus{
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
	trackJobs, err := job.GetJobs(api.ReadDB, job.JobFilter{
		ParentID: utils.Ptr(jobId),
	}, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ct := appContext.GetConvertContext()
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

		track, err := db.GetTrack(api.ReadDB, jobPayload.TrackID)
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
	var play APITrackPlayPost
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

func CorsMiddleware(next http.Handler) http.Handler {
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

func CacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}
