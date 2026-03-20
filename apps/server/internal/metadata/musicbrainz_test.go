package metadata

import "testing"

func TestNewMusicBrainzClient_SetsHTTPTimeout(t *testing.T) {
	client := NewMusicBrainzClient("")
	if client == nil || client.HTTPClient == nil {
		t.Fatal("expected musicbrainz client and HTTP client")
	}
	if client.HTTPClient.Timeout != musicBrainzHTTPTimeout {
		t.Fatalf("timeout = %v, want %v", client.HTTPClient.Timeout, musicBrainzHTTPTimeout)
	}
}

func TestBestMusicBrainzRecording_RejectsWeakGenericMatch(t *testing.T) {
	info := MusicInfo{Title: "Track 01"}
	recordings := []musicBrainzRecording{
		{
			ID:    "rec-1",
			Score: 100,
			Title: "Track 01",
		},
	}
	if got := bestMusicBrainzRecording(info, recordings); got != nil {
		t.Fatalf("expected weak generic hit to be rejected, got %+v", *got)
	}
}

func TestBestMusicBrainzRecording_AcceptsGenericMatchWithArtistAndAlbum(t *testing.T) {
	info := MusicInfo{
		Title:  "Track 01",
		Artist: "Artist Name",
		Album:  "Album Name",
	}
	recordings := []musicBrainzRecording{
		{
			ID:    "rec-1",
			Score: 95,
			Title: "Track 01",
			ArtistCredit: []musicBrainzNameCredit{
				{Artist: musicBrainzArtistRef{Name: "Artist Name"}},
			},
			Releases: []musicBrainzRelease{
				{Title: "Album Name"},
			},
		},
	}
	got := bestMusicBrainzRecording(info, recordings)
	if got == nil || got.ID != "rec-1" {
		t.Fatalf("expected confident match, got %+v", got)
	}
}
