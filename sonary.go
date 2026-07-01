package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sonary/internal/config"
	"sonary/internal/database"
	"sonary/internal/job"
	"sonary/internal/lib"
	"sonary/internal/websocket"
	"sonary/utils"
	"strconv"
	"syscall"
	"time"
)

type API struct {
	db *sql.DB
	//Store lib.Store
	//Logger     Logger
	//Config     Config
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

	tracks, hasNext, err := database.GetTracks(api.db, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiTracks := make([]lib.APITrack, len(tracks))
	for i, track := range tracks {
		apiTracks[i] = track.ToAPI()
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

	track, err := database.GetTrack(api.db, id)
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

	err = database.UpdateTrack(api.db, id, lib.TrackUpdateParams{
		Like: utils.Ptr(t.Like),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	track, err := database.GetTrack(api.db, id)
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

	artists, hasNext, err := database.GetArtists(api.db, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiArtists := make([]lib.APIArtist, len(artists))
	for i, artist := range artists {
		apiArtists[i] = artist.ToAPI()
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

	artist, err := database.GetArtist(api.db, lib.ArtistsGetParams{ID: utils.Ptr(id)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiArtist := lib.APIArtistSingle{
		APIStatus: lib.APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		APIArtist: artist.ToAPI(),
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

	albums, hasNext, err := database.GetAlbums(api.db, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiAlbums := make([]lib.APIAlbum, len(albums))
	for i, album := range albums {
		apiAlbums[i] = album.ToAPI()
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

	album, err := database.GetAlbum(api.db, lib.AlbumsGetParams{ID: utils.Ptr(id)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tracks, _, err := database.GetTracks(api.db, lib.TracksGetParams{
		AlbumID: utils.Ptr(album.ID),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiAlbum)
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

func main() {
	cfg := config.GetConfig()

	db := database.GetDB()
	defer db.Close()

	// Create a background context that listens for system shutdown signals (Ctrl+C)
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()

	// Start the worker pool
	workerCount := cfg.WorkerCount
	log.Printf("Starting worker pool with %d concurrent workers...", workerCount)
	job.StartWorkerPool(workerCtx, db, workerCount)

	log.Println("Starting sync directories ...")
	_, err := job.Enqueue(db, job.TaskSyncDirectories, nil)
	if err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}

	api := &API{
		db: db,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/tracks", api.GetTracks)
	mux.HandleFunc("GET /api/v1/tracks/{id}", api.GetTrack)
	mux.HandleFunc("PUT /api/v1/tracks/{id}", api.UpdateTrack)
	mux.HandleFunc("GET /api/v1/artists", api.GetArtists)
	mux.HandleFunc("GET /api/v1/artists/{id}", api.GetArtist)
	mux.HandleFunc("GET /api/v1/albums", api.GetAlbums)
	mux.HandleFunc("GET /api/v1/albums/{id}", api.GetAlbum)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("internal/templates/index.html"))
		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	})

	// Serve files from the "./static" directory at the "/static/" URL path
	fileServer := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

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
