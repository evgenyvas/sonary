package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sonary/internal/config"
	"sonary/internal/database"
	"sonary/internal/job"
	"sonary/internal/lib"
	"sonary/internal/track"
	"sonary/utils"
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
	var input lib.APIPathPost
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println(input.Path)
	//path := r.PathValue("path")

	//notes, err := api.Store.Notes(path)
	//if err != nil {
	//http.Error(w, err.Error(), http.StatusBadRequest)
	//return
	//}

	tracks := []lib.TrackOrDirectory{}

	apiTracks := make([]lib.APITrackOrDirectory, len(tracks))
	for i, track := range tracks {
		apiTracks[i] = lib.ToAPI(track)
	}
	apiNotesList := lib.APITrackList{
		APIStatus: lib.APIStatus{
			Status:  http.StatusOK,
			Message: "ok",
		},
		Items: apiTracks,
	}

	w.Header().Set("Content-Type", "application/json")
	if js, err := json.Marshal(apiNotesList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Write(js)
	}
}

func (api *API) ScanStatus(w http.ResponseWriter, r *http.Request) {
	var input lib.APIPathPost
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	payloadBytes, err := json.Marshal(input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	job, err := job.Get(api.db, job.JobFilter{
		TaskType: utils.Ptr(lib.TaskIndexTrackScan),
		Payload:  utils.Ptr(string(payloadBytes)),
		Status:   utils.Ptr([]string{job.StatusPending, job.StatusRunning, job.StatusFailed}),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiScan := lib.APIScan{
		CreatedAt: job.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: job.UpdatedAt.Format("2006-01-02 15:04:05"),
		Status:    job.Status,
		Message:   job.ErrorMessage.String,
		Result:    string(job.Result),
	}

	//percent := float64(stats.ScannedFiles.Load()) /
	//float64(stats.TotalFiles) * 100

	w.Header().Set("Content-Type", "application/json")
	if js, err := json.Marshal(apiScan); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Write(js)
	}
}

func (api *API) ScanStart(w http.ResponseWriter, r *http.Request) {
	var input lib.APIPathPost
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jobId, err := job.Enqueue(api.db, lib.TaskIndexTrackScan, input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiScan := lib.APIScan{
		ID:        int(jobId),
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		Status:    job.StatusPending,
		Message:   "Scan job successfully added",
	}

	w.Header().Set("Content-Type", "application/json")
	if js, err := json.Marshal(apiScan); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Write(js)
	}
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

func mains() {
	cfg := config.GetConfig()

	db := database.GetDB()
	defer db.Close()

	// Create a background context that listens for system shutdown signals (Ctrl+C)
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()

	// Start the worker pool
	workerCount := 1
	log.Printf("Starting worker pool with %d concurrent workers...", workerCount)
	job.StartWorkerPool(workerCtx, db, workerCount)

	api := &API{
		db: db,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/tracks/{path...}", api.GetTracks)
	mux.HandleFunc("GET /api/v1/scan/status", api.ScanStatus)
	mux.HandleFunc("POST /api/v1/scan/start", api.ScanStart)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("internal/templates/index.html"))
		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	})

	// Serve files from the "./static" directory at the "/static/" URL path
	fileServer := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

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

func main() {
	cfg := config.GetConfig()

	db := database.GetDB()
	defer db.Close()

	fmt.Println(cfg.RootPath)

	track.ScanLibrary(db, cfg.RootPath)
}
