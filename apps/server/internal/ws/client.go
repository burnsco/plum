package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request, checkOrigin func(*http.Request) bool) error {
	upgrader := websocket.Upgrader{
		CheckOrigin: checkOrigin,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade ws: %v", err)
		return err
	}
	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 16),
	}
	hub.Register(client)

	// Send welcome message
	welcome, _ := json.Marshal(map[string]string{
		"type":    "welcome",
		"message": "connected to plum",
	})
	client.send <- welcome

	go client.writeLoop()
	go client.readLoop()
	return nil
}

func (c *Client) readLoop() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// Very simple protocol: only support {"action":"ping"} for now.
		var payload map[string]string
		if err := json.Unmarshal(msg, &payload); err == nil {
			if payload["action"] == "ping" {
				pong, _ := json.Marshal(map[string]string{
					"type": "pong",
				})
				c.send <- pong
			}
		}
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
