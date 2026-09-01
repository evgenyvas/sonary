package main

import (
	"context"
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sonary/internal/api"
	"sonary/internal/config"
	"sonary/internal/database"
	"sonary/internal/job"
	"sonary/internal/websocket"
	"syscall"
	"time"
)

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

	a := &api.API{
		ReadDB:  readDB,
		WriteDB: writeDB,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/tracks", a.GetTracks)
	mux.HandleFunc("GET /api/v1/tracks/{id}", a.GetTrack)
	mux.HandleFunc("PUT /api/v1/tracks/{id}", a.UpdateTrack)
	mux.HandleFunc("GET /api/v1/artists", a.GetArtists)
	mux.HandleFunc("GET /api/v1/artists/{id}", a.GetArtist)
	mux.HandleFunc("GET /api/v1/albums", a.GetAlbums)
	mux.HandleFunc("GET /api/v1/albums/{id}", a.GetAlbum)
	mux.HandleFunc("POST /api/v1/convert/start", a.StartConvert)
	mux.HandleFunc("POST /api/v1/convert/download/{jobId}", a.ConvertDownload)
	mux.HandleFunc("POST /api/v1/play", a.PlayInFoobar)

	if cfg.ServeFrontend {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			tmpl := template.Must(template.ParseFiles("templates/index.html"))
			if err := tmpl.Execute(w, nil); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
		})
		// serve files from the "./static" directory at the "/static/" URL path
		fileServer := http.FileServer(http.Dir("./static"))
		mux.Handle("/static/", http.StripPrefix("/static", fileServer))
		log.Println("Web frontend enabled")
	} else {
		log.Println("Running in API-only mode")
	}

	// serve generated image thumbnails
	imageServer := http.FileServer(http.Dir(cfg.CacheDir))

	mux.Handle(
		"/api/v1/images/",
		api.CacheMiddleware(
			http.StripPrefix("/api/v1/images/", imageServer),
		),
	)

	// WebSocket
	hub := websocket.GetHub()
	mux.HandleFunc("/ws", websocket.WsEndpoint)
	go hub.Run()

	var handler http.Handler
	if cfg.AppEnv == "dev" {
		handler = api.CorsMiddleware(mux)
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
	log.Printf("Starting web server on %v", cfg.Host)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP server failed to start: %v", err)
	}

	// Active Post-Shutdown Cleanup
	// This code runs AFTER the server is completely shut down and workers are canceled.
	log.Println("Performing final system cleanups...")
	time.Sleep(1 * time.Second) // Small buffer to let everything settle

	log.Println("Application stopped cleanly.")
}
