package ws

import (
	"testing"
	"time"

	"plum/internal/db"
)

func TestHubBroadcastToUser(t *testing.T) {
	hub := NewHub()
	done := make(chan struct{})
	go func() {
		hub.Run()
		close(done)
	}()
	t.Cleanup(func() {
		hub.Close()
		<-done
	})

	userOne := &Client{id: "user-one", user: &db.User{ID: 1}, send: make(chan []byte, 1)}
	userTwo := &Client{id: "user-two", user: &db.User{ID: 2}, send: make(chan []byte, 1)}
	hub.Register(userOne)
	hub.Register(userTwo)

	hub.BroadcastToUser(2, []byte("scan-update"))

	select {
	case msg := <-userTwo.send:
		if string(msg) != "scan-update" {
			t.Fatalf("message = %q", string(msg))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for targeted message")
	}

	select {
	case msg := <-userOne.send:
		t.Fatalf("unexpected message for other user: %q", string(msg))
	case <-time.After(100 * time.Millisecond):
	}
}
