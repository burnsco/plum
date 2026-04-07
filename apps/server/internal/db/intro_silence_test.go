package db

import "testing"

func TestParseSilenceDetectOutput(t *testing.T) {
	raw := `
[silencedetect @ 0xaaa] silence_start: 1.2
[silencedetect @ 0xaaa] silence_end: 2.5 | silence_duration: 1.3
[silencedetect @ 0xaaa] silence_start: 88.501
[silencedetect @ 0xaaa] silence_end: 90.25 | silence_duration: 1.749
`
	iv := parseSilenceDetectOutput(raw)
	if len(iv) != 2 {
		t.Fatalf("len=%d want 2", len(iv))
	}
	if iv[0].start != 1.2 || iv[0].end != 2.5 || iv[0].duration != 1.3 {
		t.Fatalf("first interval %+v", iv[0])
	}
	if iv[1].start != 88.501 || iv[1].end != 90.25 || iv[1].duration != 1.749 {
		t.Fatalf("second interval %+v", iv[1])
	}
}

func TestPickIntroEndFromSilence(t *testing.T) {
	iv := []silenceInterval{
		{start: 5, end: 6, duration: 1},
		{start: 30, end: 31.2, duration: 1.2},
	}
	end, ok := pickIntroEndFromSilence(iv, 3600)
	if !ok || end != 31.2 {
		t.Fatalf("got (%v,%v) want (31.2,true)", end, ok)
	}
	_, ok = pickIntroEndFromSilence([]silenceInterval{{start: 10, end: 11, duration: 1}}, 3600)
	if ok {
		t.Fatal("expected reject: start before min")
	}
	_, ok = pickIntroEndFromSilence([]silenceInterval{{start: 30, end: 15, duration: 2}}, 3600)
	if ok {
		t.Fatal("expected reject: end before min end sec")
	}
	_, ok = pickIntroEndFromSilence([]silenceInterval{{start: 30, end: 100, duration: 0.1}}, 3600)
	if ok {
		t.Fatal("expected reject: duration below threshold")
	}
	end, ok = pickIntroEndFromSilence([]silenceInterval{{start: 40, end: 42, duration: 2}}, 45)
	if ok {
		t.Fatal("expected reject: too close to file end")
	}
}
