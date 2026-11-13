// Package ws implements the WebSocket hub that broadcasts geofence alert
// events to every connected frontend client.
package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong from the peer.
	pongWait = 60 * time.Second
	// Send pings to the peer with this period (must be less than pongWait).
	pingPeriod = 30 * time.Second
	// Outbound buffer size per client.
	sendBufferSize = 64
	// Hub broadcast buffer size.
	broadcastBufferSize = 256
)

// Hub tracks connected clients and fans alert events out to all of them.
type Hub struct {
	mu        sync.RWMutex
	clients   map[*Client]bool
	broadcast chan []byte
	done      chan struct{}
}

// Client is a single connected WebSocket peer.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	once sync.Once
}

// NewHub creates an empty hub. Call Run to start the broadcast loop.
func NewHub() *Hub {
	return &Hub{
		clients:   make(map[*Client]bool),
		broadcast: make(chan []byte, broadcastBufferSize),
		done:      make(chan struct{}),
	}
}

// Run processes queued broadcasts until ctx-dependent Stop is called.
func (h *Hub) Run() {
	for {
		select {
		case msg := <-h.broadcast:
			h.deliver(msg)
		case <-h.done:
			return
		}
	}
}

// Stop shuts down the hub's broadcast loop.
func (h *Hub) Stop() {
	select {
	case <-h.done:
	default:
		close(h.done)
	}
}

// Broadcast marshals v and queues it for asynchronous delivery to every
// connected client. It never blocks the caller: if the queue is full the
// message is dropped (a slow client must not stall location updates).
func (h *Hub) Broadcast(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("ws: marshal broadcast: %v", err)
		return
	}
	select {
	case h.broadcast <- data:
	default:
		log.Printf("ws: broadcast queue full, dropping message")
	}
}

func (h *Hub) deliver(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
			// Client is too slow; drop it rather than block others.
			go c.close()
		}
	}
}

// Register wires a freshly upgraded connection into the hub and starts its
// read/write pumps. The connection is removed automatically on disconnect.
func (h *Hub) Register(conn *websocket.Conn) *Client {
	c := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, sendBufferSize),
	}
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	go c.writePump()
	go c.readPump()
	return c
}

// Unregister removes a client from the hub (idempotent).
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	if h.clients[c] {
		delete(h.clients, c)
	}
	h.mu.Unlock()
	c.close()
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (c *Client) close() {
	c.once.Do(func() {
		close(c.send)
		_ = c.conn.Close()
	})
}

// readPump drains inbound frames (clients are receive-only) and detects
// disconnects so the client can be unregistered promptly.
func (c *Client) readPump() {
	defer c.hub.Unregister(c)

	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump sends queued broadcasts and keeps the connection alive with
// periodic pings.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Upgrader upgrades HTTP connections to WebSocket. Origin checks are open
// because the assessment frontend may be served from any host; production
// deployments should restrict CheckOrigin to known domains.
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}
