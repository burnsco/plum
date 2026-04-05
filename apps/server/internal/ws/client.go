package ws

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"plum/internal/db"
)

type Client struct {
	id       string
	user     *db.User
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	doneOnce sync.Once
	done     chan struct{}
	onClose  func(*Client)
	onText   func(*Client, []byte)
}

func (c *Client) signalDone() {
	c.doneOnce.Do(func() { close(c.done) })
}

type ServeOptions struct {
	CheckOrigin func(*http.Request) bool
	User        *db.User
	OnClose     func(*Client)
	OnText      func(*Client, []byte)
}

func (c *Client) ID() string {
	return c.id
}

func (c *Client) User() *db.User {
	return c.user
}

func (c *Client) Send(msg []byte) bool {
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request, options ServeOptions) error {
	upgrader := websocket.Upgrader{
		CheckOrigin: options.CheckOrigin,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Debug("websocket upgrade failed", "error", err)
		return err
	}
	client := &Client{
		id:      newClientID(),
		user:    options.User,
		hub:     hub,
		conn:    conn,
		send:    make(chan []byte, clientSendBuffer),
		done:    make(chan struct{}),
		onClose: options.OnClose,
		onText:  options.OnText,
	}
	if !hub.Register(client) {
		_ = conn.Close()
		return errHubStopped
	}

	// Send welcome message
	welcome, _ := json.Marshal(map[string]string{
		"type":    "welcome",
		"message": "connected to plum",
	})
	if !client.Send(welcome) {
		client.signalDone()
		_ = conn.Close()
		return errHubStopped
	}

	go client.writeLoop()
	go client.readLoop()
	return nil
}

var errHubStopped = errors.New("hub stopped")

func (c *Client) readLoop() {
	defer func() {
		if c.onClose != nil {
			c.onClose(c)
		}
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(1024)
	// ~2× ping interval (30s) + margin so jittery networks don’t hit the deadline before the next ping/pong.
	const readDeadline = 75 * time.Second
	c.conn.SetReadDeadline(time.Now().Add(readDeadline))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(readDeadline))
		return nil
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		if c.onText != nil {
			c.onText(c, msg)
		}
		// Very simple protocol: only support {"action":"ping"} for now.
		var payload map[string]string
		if err := json.Unmarshal(msg, &payload); err == nil {
			if payload["action"] == "ping" {
				pong, _ := json.Marshal(map[string]string{
					"type": "pong",
				})
				c.Send(pong)
			}
		}
	}
}

func newClientID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "ws-" + time.Now().UTC().Format(time.RFC3339Nano)
	}
	return hex.EncodeToString(buf[:])
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-c.done:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return
		case msg := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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
