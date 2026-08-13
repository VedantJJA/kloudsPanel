// Package websocket provides connection management and broadcasting for real-time logs and terminal streams.
package websocket

import (
	"sync"
)

// LogMessage represents a single streamed log payload.
type LogMessage struct {
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"` // "stdout" | "stderr" | "build" | "system"
	Message   string `json:"message"`
}

// Hub maintains the set of active log stream connections per service/deployment.
type Hub struct {
	// Channels keyed by deploymentID or serviceID
	rooms map[string]map[*Client]bool
	mu    sync.RWMutex

	broadcast  chan RoomMessage
	register   chan Subscription
	unregister chan Subscription
}

// RoomMessage packages a target room and data.
type RoomMessage struct {
	RoomID  string
	Message LogMessage
}

// Subscription tracks client joining or leaving a room.
type Subscription struct {
	RoomID string
	Client *Client
}

// NewHub creates a new WebSocket broadcasting Hub.
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		broadcast:  make(chan RoomMessage, 256),
		register:   make(chan Subscription, 64),
		unregister: make(chan Subscription, 64),
	}
}

// Run starts the message dispatcher loop.
func (h *Hub) Run() {
	for {
		select {
		case sub := <-h.register:
			h.mu.Lock()
			clients := h.rooms[sub.RoomID]
			if clients == nil {
				clients = make(map[*Client]bool)
				h.rooms[sub.RoomID] = clients
			}
			clients[sub.Client] = true
			h.mu.Unlock()

		case sub := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.rooms[sub.RoomID]; ok {
				delete(clients, sub.Client)
				close(sub.Client.Send)
				if len(clients) == 0 {
					delete(h.rooms, sub.RoomID)
				}
			}
			h.mu.Unlock()

		case rm := <-h.broadcast:
			h.mu.RLock()
			if clients, ok := h.rooms[rm.RoomID]; ok {
				for client := range clients {
					select {
					case client.Send <- rm.Message:
					default:
						close(client.Send)
						delete(clients, client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToRoom dispatches a log message to all active listeners in a room.
func (h *Hub) BroadcastToRoom(roomID string, msg LogMessage) {
	h.broadcast <- RoomMessage{RoomID: roomID, Message: msg}
}

// Subscribe adds a client to a room.
func (h *Hub) Subscribe(roomID string, client *Client) {
	h.register <- Subscription{RoomID: roomID, Client: client}
}

// Unsubscribe removes a client from a room.
func (h *Hub) Unsubscribe(roomID string, client *Client) {
	h.unregister <- Subscription{RoomID: roomID, Client: client}
}
