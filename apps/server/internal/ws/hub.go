package ws

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
)

type registerOp struct {
	client *Client
	ack    chan bool
}

type Hub struct {
	register   chan registerOp
	unregister chan *Client
	broadcast  chan []byte
	targeted   chan targetedBroadcast
	clients    map[*Client]struct{}
	maxClients int
	runEnded   chan struct{}
	closed     atomic.Bool
}

type targetedBroadcast struct {
	userID int
	msg    []byte
}

// Hub/client channel capacities. Global broadcast fans out to every connection; larger buffer
// reduces drops under bursty updates. Per-client send queues outbound messages before writeLoop.
const (
	hubBroadcastBuffer = 128
	hubTargetedBuffer  = 64
	clientSendBuffer   = 64
)

func maxHubClientsFromEnv() int {
	raw := strings.TrimSpace(os.Getenv("PLUM_WS_MAX_CLIENTS"))
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan registerOp),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, hubBroadcastBuffer),
		targeted:   make(chan targetedBroadcast, hubTargetedBuffer),
		clients:    make(map[*Client]struct{}),
		maxClients: maxHubClientsFromEnv(),
		runEnded:   make(chan struct{}),
	}
}

func (h *Hub) Run() {
	defer close(h.runEnded)
	for {
		select {
		case op := <-h.register:
			if h.maxClients > 0 && len(h.clients) >= h.maxClients {
				slog.Warn("ws hub client limit reached, rejecting connection", "max_clients", h.maxClients)
				op.ack <- false
				continue
			}
			h.clients[op.client] = struct{}{}
			op.ack <- true
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				c.signalDone()
			}
		case msg, ok := <-h.broadcast:
			if !ok {
				h.shutdownAllClients()
				return
			}
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					delete(h.clients, c)
					c.signalDone()
				}
			}
		case targeted, ok := <-h.targeted:
			if !ok {
				h.shutdownAllClients()
				return
			}
			for c := range h.clients {
				if c.User() == nil || c.User().ID != targeted.userID {
					continue
				}
				select {
				case c.send <- targeted.msg:
				default:
					delete(h.clients, c)
					c.signalDone()
				}
			}
		}
	}
}

func (h *Hub) shutdownAllClients() {
	for c := range h.clients {
		c.signalDone()
		delete(h.clients, c)
	}
}

func (h *Hub) Broadcast(msg []byte) {
	if h.closed.Load() {
		return
	}
	select {
	case h.broadcast <- msg:
	default:
		slog.Warn("ws broadcast buffer full, dropping message")
	}
}

func (h *Hub) BroadcastToUser(userID int, msg []byte) {
	if h.closed.Load() {
		return
	}
	select {
	case h.targeted <- targetedBroadcast{userID: userID, msg: msg}:
	default:
		slog.Warn("ws targeted broadcast buffer full, dropping message", "user_id", userID)
	}
}

// Register blocks until the client is registered or the hub's Run loop has exited. Returns false
// if the hub has stopped (caller should close the WebSocket). Must not rely only on Close()'s
// closed flag: Run may still be exiting while register is processed.
func (h *Hub) Register(c *Client) bool {
	ack := make(chan bool, 1)
	select {
	case h.register <- registerOp{client: c, ack: ack}:
		return <-ack
	case <-h.runEnded:
		return false
	}
}

func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

func (h *Hub) Close() {
	if h.closed.Swap(true) {
		return
	}
	close(h.broadcast)
	close(h.targeted)
}
