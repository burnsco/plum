package db

import "testing"

func TestEmbeddedSubtitleWebVTTDeliveryEligible(t *testing.T) {
	falseVal := false
	trueVal := true
	cases := []struct {
		name string
		sub  EmbeddedSubtitle
		want bool
	}{
		{"subrip", EmbeddedSubtitle{Codec: "subrip"}, true},
		{"empty codec", EmbeddedSubtitle{}, true},
		{"supported false", EmbeddedSubtitle{Codec: "subrip", Supported: &falseVal}, false},
		{"supported true", EmbeddedSubtitle{Codec: "subrip", Supported: &trueVal}, true},
		{"pgs", EmbeddedSubtitle{Codec: "hdmv_pgs_subtitle"}, false},
		{"pgs supported true still bitmap", EmbeddedSubtitle{Codec: "hdmv_pgs_subtitle", Supported: &trueVal}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EmbeddedSubtitleWebVTTDeliveryEligible(tc.sub); got != tc.want {
				t.Fatalf("got %v want %v for %#v", got, tc.want, tc.sub)
			}
		})
	}
}

func TestPlaybackEmbeddedSubtitles(t *testing.T) {
	in := []EmbeddedSubtitle{
		{StreamIndex: 1, Codec: "subrip"},
		{StreamIndex: 2, Codec: "hdmv_pgs_subtitle"},
	}
	got := PlaybackEmbeddedSubtitles(in)
	if len(got) != 1 || got[0].StreamIndex != 1 {
		t.Fatalf("got %#v", got)
	}
	if PlaybackEmbeddedSubtitles(nil) != nil {
		t.Fatal("nil in should be nil out")
	}
}
