package viewer

import (
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub       *Hub
	sessionID string
	conn      *websocket.Conn
	send      chan []byte
}

type Hub struct {
	// Registered clients mapped by session ID
	clients    map[string]map[*Client]bool
	clientsMu  sync.RWMutex
	Broadcast  chan BroadcastMessage
	register   chan *Client
	unregister chan *Client
}

type BroadcastMessage struct {
	SessionID string
	Payload   []byte
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan BroadcastMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[string]map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clientsMu.Lock()
			if h.clients[client.sessionID] == nil {
				h.clients[client.sessionID] = make(map[*Client]bool)
			}
			h.clients[client.sessionID][client] = true
			h.clientsMu.Unlock()
			fmt.Printf("WebSocket client connected to session %s\n", client.sessionID)

		case client := <-h.unregister:
			h.clientsMu.Lock()
			if clients, ok := h.clients[client.sessionID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.sessionID)
					}
				}
			}
			h.clientsMu.Unlock()
			fmt.Printf("WebSocket client disconnected from session %s\n", client.sessionID)

		case message := <-h.Broadcast:
			h.clientsMu.RLock()
			if clients, ok := h.clients[message.SessionID]; ok {
				for client := range clients {
					select {
					case client.send <- message.Payload:
					default:
						// If send buffer is full, assume client is dead
						close(client.send)
						delete(h.clients[message.SessionID], client)
					}
				}
			}
			h.clientsMu.RUnlock()
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		message, ok := <-c.send
		if !ok {
			// The hub closed the channel.
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		w, err := c.conn.NextWriter(websocket.TextMessage)
		if err != nil {
			return
		}
		w.Write(message)

		if err := w.Close(); err != nil {
			return
		}
	}
}
