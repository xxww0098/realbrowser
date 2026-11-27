package process

import (
	"maps"
	"sync"
)

// Registry tracks the running instances of one host. Its zero value is ready for use.
type Registry[K comparable, V any] struct {
	mu      sync.RWMutex
	entries map[K]V
}

func (r *Registry[K, V]) Set(id K, entry V) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = make(map[K]V)
	}
	r.entries[id] = entry
}

func (r *Registry[K, V]) Get(id K) (V, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[id]
	return entry, ok
}

// RemoveIf returns the current entry and whether it was removed. An exit
// callback can match its own process, so a later launch retains its registration.
func (r *Registry[K, V]) RemoveIf(id K, match func(V) bool) (V, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[id]
	if ok && (match == nil || match(entry)) {
		delete(r.entries, id)
		return entry, true
	}
	return entry, false
}

func (r *Registry[K, V]) Snapshot() map[K]V {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return maps.Clone(r.entries)
}

func (r *Registry[K, V]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}
