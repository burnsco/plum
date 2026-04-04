package db

import "testing"

func TestIntroChapterRangeFromProbes(t *testing.T) {
	t.Parallel()
	start, end, ok := IntroChapterRangeFromProbes([]chapterProbe{
		{startSec: 0, endSec: 30, title: "Previously on..."},
		{startSec: 30, endSec: 120, title: "Intro"},
	})
	if !ok || start != 30 || end != 120 {
		t.Fatalf("got ok=%v start=%v end=%v", ok, start, end)
	}

	_, _, ok2 := IntroChapterRangeFromProbes([]chapterProbe{
		{startSec: 0, endSec: 90, title: "Chapter 1"},
	})
	if ok2 {
		t.Fatal("expected no match")
	}

	start3, end3, ok3 := IntroChapterRangeFromProbes([]chapterProbe{
		{startSec: 5.5, endSec: 88.25, title: "Opening Sequence"},
	})
	if !ok3 || start3 != 5.5 || end3 != 88.25 {
		t.Fatalf("got ok=%v start=%v end=%v", ok3, start3, end3)
	}
}
