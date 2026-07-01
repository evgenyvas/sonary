// Package websocket
package websocket

import (
	"log"
	"net/http"
	"sonary/internal/lib"
	"sonary/utils"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub/Broadcast pattern

type ProgressEvent struct {
	Type     string `json:"type"`
	Progress int    `json:"progress"` // Percentage from 0 to 100
}

type Hub struct {
	Clients    map[*websocket.Conn]bool
	Broadcast  chan ProgressEvent
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.Mutex
}

var (
	instance *Hub
	once     sync.Once
)

func GetHub() *Hub {
	once.Do(func() {
		instance = &Hub{
			Broadcast:  make(chan ProgressEvent),
			register:   make(chan *websocket.Conn),
			unregister: make(chan *websocket.Conn),
			Clients:    make(map[*websocket.Conn]bool),
		}
	})
	return instance
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.Clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				client.Close()
			}
			h.mu.Unlock()
		case event := <-h.Broadcast:
			h.mu.Lock()
			for client := range h.Clients {
				// Gorilla WriteJSON handles concurrent-safe encoding to connection
				err := client.WriteJSON(event)
				if err != nil {
					log.Printf("Client disconnected implicitly: %v", err)
					client.Close()
					delete(h.Clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
		//origin := r.Header.Get("Origin")
		//return origin == "<http://yourdomain.com>"
	},
}

func WsEndpoint(w http.ResponseWriter, r *http.Request) {
	hub := GetHub()
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	hub.register <- ws
	defer func() { hub.unregister <- ws }()

	// after client connect get progress percent
	ct := lib.GetImportContext(false)
	if ct.Progress.Total > 0 {
		processed := int(ct.Progress.Processed.Load())
		hub.Broadcast <- ProgressEvent{
			Type:     lib.EventProgressUpdate,
			Progress: utils.GetPercent(processed, ct.Progress.Total),
		}
	}

	// Keep-alive loop to detect client closures
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}
}
