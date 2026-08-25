package realtime

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

type Message struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Time time.Time       `json:"time"`
	Data json.RawMessage `json:"data"`
}
type subscription struct{ ch chan Message }
type Broker struct {
	mu      sync.RWMutex
	next    uint64
	subs    map[uint64]*subscription
	dropped atomic.Uint64
}

func New() *Broker { return &Broker{subs: make(map[uint64]*subscription)} }
func (b *Broker) Subscribe(buffer int) (<-chan Message, func()) {
	if buffer <= 0 {
		buffer = 32
	}
	b.mu.Lock()
	b.next++
	id := b.next
	s := &subscription{ch: make(chan Message, buffer)}
	b.subs[id] = s
	b.mu.Unlock()
	var once sync.Once
	return s.ch, func() {
		once.Do(func() {
			b.mu.Lock()
			if cur := b.subs[id]; cur != nil {
				delete(b.subs, id)
				close(cur.ch)
			}
			b.mu.Unlock()
		})
	}
}
func (b *Broker) Publish(m Message) {
	if m.Time.IsZero() {
		m.Time = time.Now().UTC()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, s := range b.subs {
		select {
		case s.ch <- m:
		default:
			b.dropped.Add(1)
		}
	}
}
func (b *Broker) Stats() (subscribers int, dropped uint64) {
	b.mu.RLock()
	subscribers = len(b.subs)
	b.mu.RUnlock()
	return subscribers, b.dropped.Load()
}
