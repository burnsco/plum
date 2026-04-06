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

	// Adjacent intro chapters should merge into a single range.
	start4, end4, ok4 := IntroChapterRangeFromProbes([]chapterProbe{
		{startSec: 0, endSec: 30, title: "Opening"},
		{startSec: 30, endSec: 90, title: "Opening Theme"},
	})
	if !ok4 || start4 != 0 || end4 != 90 {
		t.Fatalf("adjacent merge: got ok=%v start=%v end=%v", ok4, start4, end4)
	}

	// Non-intro chapter between two intro chapters stops the merge.
	start5, end5, ok5 := IntroChapterRangeFromProbes([]chapterProbe{
		{startSec: 0, endSec: 30, title: "Intro"},
		{startSec: 30, endSec: 60, title: "Scene 1"},
		{startSec: 60, endSec: 90, title: "Intro reprise"},
	})
	if !ok5 || start5 != 0 || end5 != 30 {
		t.Fatalf("gap break: got ok=%v start=%v end=%v", ok5, start5, end5)
	}

	// Merged range exceeding max duration is rejected.
	_, _, ok6 := IntroChapterRangeFromProbes([]chapterProbe{
		{startSec: 0, endSec: 400, title: "OP part 1"},
		{startSec: 400, endSec: 800, title: "OP part 2"},
	})
	if ok6 {
		t.Fatal("expected merged range exceeding max duration to be rejected")
	}

	// Oversized intro chapter should be skipped; a valid one after it should be found.
	start7, end7, ok7 := IntroChapterRangeFromProbes([]chapterProbe{
		{startSec: 0, endSec: 700, title: "Intro"},
		{startSec: 700, endSec: 790, title: "Opening"},
	})
	if !ok7 || start7 != 700 || end7 != 790 {
		t.Fatalf("oversized skip: got ok=%v start=%v end=%v", ok7, start7, end7)
	}
}
