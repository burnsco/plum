package ws

import (
	"log"
)

type Hub struct {
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	targeted   chan targetedBroadcast
	clients    map[*Client]struct{}
}

type targetedBroadcast struct {
	userID int
	msg    []byte
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 16),
		targeted:   make(chan targetedBroadcast, 16),
		clients:    make(map[*Client]struct{}),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = struct{}{}
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
		case msg, ok := <-h.broadcast:
			if !ok {
				for c := range h.clients {
					close(c.send)
					delete(h.clients, c)
				}
				return
			}
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					delete(h.clients, c)
					close(c.send)
				}
			}
		case targeted := <-h.targeted:
			for c := range h.clients {
				if c.User() == nil || c.User().ID != targeted.userID {
					continue
				}
				select {
				case c.send <- targeted.msg:
				default:
					delete(h.clients, c)
					close(c.send)
				}
			}
		}
	}
}

func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		log.Printf("ws broadcast buffer full, dropping message")
	}
}

func (h *Hub) BroadcastToUser(userID int, msg []byte) {
	select {
	case h.targeted <- targetedBroadcast{userID: userID, msg: msg}:
	default:
		log.Printf("ws targeted broadcast buffer full, dropping message user=%d", userID)
	}
}

func (h *Hub) Register(c *Client) {
	h.register <- c
}

func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

func (h *Hub) Close() {
	close(h.broadcast)
}
