// Package websocket
package websocket

import (
	"log"
	"net/http"
	"sonary/internal/context"
	"sonary/utils"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub/Broadcast pattern

type WebSocketEvent interface {
	GetUserID() string
}

type BaseEvent struct {
	UserID string `json:"-"` // minus hides field from the client's JSON
}

func (b BaseEvent) GetUserID() string {
	return b.UserID
}

type ProgressEvent struct {
	BaseEvent
	Type     string `json:"type"`
	Progress int    `json:"progress"` // Percentage from 0 to 100
}

type ProgressConvertEvent struct {
	BaseEvent
	Type      string `json:"type"`
	Total     int    `json:"total"`
	Processed int    `json:"processed"`
	Progress  int    `json:"progress"` // Percentage from 0 to 100
}

const ConvertStatusProcessing = "PROCESSING"
const ConvertStatusCompleted = "COMPLETED"
const ConvertStatusFailed = "FAILED"

type ProgressTrackConvertEvent struct {
	BaseEvent
	Type       string `json:"type"`
	Progress   int    `json:"progress"` // Percentage from 0 to 100
	Status     string `json:"status"`
	Error      string `json:"error"`
	TrackID    int    `json:"track_id"`
	TrackTitle string `json:"track_title"`
}

const MessageVariantError = "MESSAGE_ERROR"

type MessageEvent struct {
	BaseEvent
	Variant string `json:"variant"`
	Message string `json:"message"`
}

type clientSession struct {
	userID string
	conn   *websocket.Conn
}

type Hub struct {
	Clients    map[string]*websocket.Conn
	Send       chan WebSocketEvent
	register   chan clientSession
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
			Clients:    make(map[string]*websocket.Conn),
			Send:       make(chan WebSocketEvent),
			register:   make(chan clientSession),
			unregister: make(chan *websocket.Conn),
		}
	})
	return instance
}

func (h *Hub) Run() {
	for {
		select {
		case session := <-h.register:
			h.mu.Lock()
			h.Clients[session.userID] = session.conn
			h.mu.Unlock()
		case clientConn := <-h.unregister:
			h.mu.Lock()
			for id, conn := range h.Clients {
				if conn == clientConn {
					delete(h.Clients, id)
					conn.Close()
					break
				}
			}
			h.mu.Unlock()
		case event := <-h.Send:
			h.mu.Lock()

			userID := event.GetUserID()

			// send only to specific user
			if userID != "" {
				if client, ok := h.Clients[userID]; ok {
					err := client.WriteJSON(event)
					if err != nil {
						log.Printf("Target client disconnected: %v", err)
						client.Close()
						delete(h.Clients, userID)
					}
				}
				h.mu.Unlock()
				continue
			}

			// broadcast
			for id, client := range h.Clients {
				err := client.WriteJSON(event)
				if err != nil {
					log.Printf("Client disconnected implicitly: %v", err)
					client.Close()
					delete(h.Clients, id)
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
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "Unauthorized: missing userId", http.StatusBadRequest)
		return
	}

	hub := GetHub()
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	hub.register <- clientSession{userID: userID, conn: ws}
	defer func() { hub.unregister <- ws }()

	// after client connect get progress percent
	ct := context.GetImportContext()
	if ct.Progress.Total > 0 {
		processed := int(ct.Progress.Processed.Load())
		hub.Send <- ProgressEvent{
			BaseEvent: BaseEvent{UserID: userID},
			Type:      context.EventImportProgressUpdate,
			Progress:  utils.GetPercent(processed, ct.Progress.Total),
		}
	}

	// Keep-alive loop to detect client closures
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}
}
