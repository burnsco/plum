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

	userOne := &Client{id: "user-one", user: &db.User{ID: 1}, send: make(chan []byte, 1), done: make(chan struct{})}
	userTwo := &Client{id: "user-two", user: &db.User{ID: 2}, send: make(chan []byte, 1), done: make(chan struct{})}
	if !hub.Register(userOne) || !hub.Register(userTwo) {
		t.Fatal("register client")
	}

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

func TestHubTargetedDoesNotDisconnectOnFullClientBuffer(t *testing.T) {
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

	client := &Client{id: "slow", user: &db.User{ID: 1}, send: make(chan []byte, 1), done: make(chan struct{})}
	if !hub.Register(client) {
		t.Fatal("register client")
	}

	client.send <- []byte("stale")

	hub.BroadcastToUser(1, []byte("fresh"))
	// Hub processes targeted sends asynchronously; wait so eviction + enqueue run before we read.
	time.Sleep(50 * time.Millisecond)
	select {
	case msg := <-client.send:
		if string(msg) != "fresh" {
			t.Fatalf("first message after eviction = %q, want fresh", string(msg))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fresh message")
	}

	hub.BroadcastToUser(1, []byte("second"))
	time.Sleep(50 * time.Millisecond)
	select {
	case msg := <-client.send:
		if string(msg) != "second" {
			t.Fatalf("second message = %q", string(msg))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second message — client may have been disconnected")
	}
}
