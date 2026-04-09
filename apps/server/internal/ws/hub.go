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
	stop       chan struct{}
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
	hubBroadcastBuffer = 256
	hubTargetedBuffer  = 128
	clientSendBuffer   = 128
)

// enqueueFanout delivers a hub fan-out frame to a client. If the per-client queue is full, evict
// one oldest queued message (typically stale under scan/playback bursts) instead of disconnecting.
func enqueueFanout(c *Client, msg []byte) {
	select {
	case c.send <- msg:
		return
	default:
	}
	select {
	case <-c.send:
	default:
	}
	select {
	case c.send <- msg:
	default:
		slog.Debug("ws: client send queue saturated after eviction, dropping fan-out frame", "client_id", c.ID())
	}
}

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
		stop:       make(chan struct{}),
		clients:    make(map[*Client]struct{}),
		maxClients: maxHubClientsFromEnv(),
		runEnded:   make(chan struct{}),
	}
}

func (h *Hub) Run() {
	defer close(h.runEnded)
	for {
		select {
		case <-h.stop:
			h.shutdownAllClients()
			return
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
		case msg := <-h.broadcast:
			for c := range h.clients {
				enqueueFanout(c, msg)
			}
		case targeted := <-h.targeted:
			for c := range h.clients {
				if c.User() == nil || c.User().ID != targeted.userID {
					continue
				}
				enqueueFanout(c, targeted.msg)
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
	select {
	case <-h.runEnded:
		return
	case h.broadcast <- msg:
	default:
		slog.Warn("ws broadcast buffer full, dropping message")
	}
}

func (h *Hub) BroadcastToUser(userID int, msg []byte) {
	select {
	case <-h.runEnded:
		return
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
	select {
	case h.unregister <- c:
	case <-h.runEnded:
	}
}

func (h *Hub) Close() {
	if h.closed.Swap(true) {
		return
	}
	close(h.stop)
}
