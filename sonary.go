package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sonary/internal/config"
	"sonary/internal/lib"
)

type API struct {
	//Store lib.Store
	//Logger     Logger
	//Config     Config
	//Mailer     Mailer
	//Cache      Cache
}

func (api *API) GetTracks(w http.ResponseWriter, r *http.Request) {
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

	//storage := lib.GetStorageService()
	api := &API{
		//Store: storage.Store,
	}

	m := http.NewServeMux()
	m.HandleFunc("GET /api/v1/tracks/{path...}", api.GetTracks)

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("internal/templates/index.html"))
		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	})

	// Serve files from the "./static" directory at the "/static/" URL path
	fileServer := http.FileServer(http.Dir("./static"))
	m.Handle("/static/", http.StripPrefix("/static", fileServer))

	var handler http.Handler
	if cfg.AppEnv == "dev" {
		handler = corsMiddleware(m)
	} else {
		handler = m
	}

	log.Fatal(http.ListenAndServe(cfg.Host, handler))
}
