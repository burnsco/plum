package metadata

import (
	"sync"
	"testing"
)

func TestTMDBIntLRU_evictsOldest(t *testing.T) {
	t.Parallel()
	const max = 3
	l := newTMDBIntLRU[int](max)
	var mu sync.Mutex // mirror TMDBClient usage
	mu.Lock()
	l.put(1, 10)
	l.put(2, 20)
	l.put(3, 30)
	mu.Unlock()

	mu.Lock()
	if _, ok := l.get(1); !ok {
		t.Fatal("expected key 1")
	}
	l.put(4, 40)
	if _, ok := l.get(2); ok {
		t.Fatal("expected key 2 evicted (LRU)")
	}
	for _, id := range []int{1, 3, 4} {
		if _, ok := l.get(id); !ok {
			t.Fatalf("expected key %d present", id)
		}
	}
	mu.Unlock()
}
