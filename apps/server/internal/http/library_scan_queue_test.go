package httpapi

import (
	"testing"

	"plum/internal/db"
)

func TestLibraryScanQueueTypePriority(t *testing.T) {
	t.Parallel()
	if libraryScanQueueTypePriority(db.LibraryTypeMovie) >= libraryScanQueueTypePriority(db.LibraryTypeTV) {
		t.Fatal("movie should sort before tv")
	}
	if libraryScanQueueTypePriority(db.LibraryTypeTV) >= libraryScanQueueTypePriority(db.LibraryTypeAnime) {
		t.Fatal("tv should sort before anime")
	}
	if libraryScanQueueTypePriority(db.LibraryTypeAnime) >= libraryScanQueueTypePriority(db.LibraryTypeMusic) {
		t.Fatal("anime should sort before music")
	}
}
