package metadata

// tmdbIntLRU is a small integer-keyed LRU used to cap TMDB in-memory detail/ID caches.
type tmdbIntLRU[V any] struct {
	max   int
	order []int
	vals  map[int]V
}

func newTMDBIntLRU[V any](max int) *tmdbIntLRU[V] {
	if max <= 0 {
		max = 1
	}
	return &tmdbIntLRU[V]{
		max:   max,
		vals:  make(map[int]V),
		order: make([]int, 0, max),
	}
}

// get returns a cached value and marks it most-recently-used. Caller must hold TMDBClient.mu.
func (l *tmdbIntLRU[V]) get(id int) (V, bool) {
	var zero V
	if l == nil || l.vals == nil {
		return zero, false
	}
	v, ok := l.vals[id]
	if !ok {
		return zero, false
	}
	for i, k := range l.order {
		if k == id {
			l.order = append(append(l.order[:i], l.order[i+1:]...), id)
			break
		}
	}
	return v, true
}

// put stores v for id, evicting the least-recently-used entry when full. Caller must hold TMDBClient.mu.
func (l *tmdbIntLRU[V]) put(id int, v V) {
	if l == nil {
		return
	}
	if l.vals == nil {
		l.vals = make(map[int]V)
		l.order = make([]int, 0, l.max)
	}
	if _, exists := l.vals[id]; exists {
		l.vals[id] = v
		for i, k := range l.order {
			if k == id {
				l.order = append(append(l.order[:i], l.order[i+1:]...), id)
				return
			}
		}
		l.order = append(l.order, id)
		return
	}
	for len(l.order) >= l.max {
		evict := l.order[0]
		l.order = l.order[1:]
		delete(l.vals, evict)
	}
	l.vals[id] = v
	l.order = append(l.order, id)
}
