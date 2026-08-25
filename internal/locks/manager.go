package locks

import "sync"

type entry struct {
	mu   sync.Mutex
	refs int
}
type Manager struct {
	mu      sync.Mutex
	entries map[string]*entry
}

func New() *Manager { return &Manager{entries: make(map[string]*entry)} }

// Lock serializes operations for one logical resource. The returned release
// function is idempotent to make deferred cleanup safe across error paths.
func (m *Manager) Lock(key string) func() {
	m.mu.Lock()
	e := m.entries[key]
	if e == nil {
		e = &entry{}
		m.entries[key] = e
	}
	e.refs++
	m.mu.Unlock()
	e.mu.Lock()
	var once sync.Once
	return func() {
		once.Do(func() {
			e.mu.Unlock()
			m.mu.Lock()
			e.refs--
			if e.refs == 0 {
				delete(m.entries, key)
			}
			m.mu.Unlock()
		})
	}
}
func (m *Manager) ActiveKeys() int { m.mu.Lock(); defer m.mu.Unlock(); return len(m.entries) }
