package events

import (
	"context"
	"sync"
	"sync/atomic"
)

type OverflowPolicy string

const DropNewest OverflowPolicy = "drop_newest"

type subscriber struct {
	ch      chan Envelope
	dropped atomic.Uint64
}
type Subscription struct {
	C      <-chan Envelope
	cancel func()
	sub    *subscriber
}

func (s Subscription) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}
func (s Subscription) Dropped() uint64 {
	if s.sub == nil {
		return 0
	}
	return s.sub.dropped.Load()
}

type Bus struct {
	mu     sync.RWMutex
	subs   map[uint64]*subscriber
	next   uint64
	closed bool
}

func NewBus() *Bus { return &Bus{subs: map[uint64]*subscriber{}} }
func (b *Bus) Subscribe(buffer int) Subscription {
	if buffer < 1 {
		buffer = 1
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		ch := make(chan Envelope)
		close(ch)
		return Subscription{C: ch}
	}
	b.next++
	id := b.next
	s := &subscriber{ch: make(chan Envelope, buffer)}
	b.subs[id] = s
	var once sync.Once
	return Subscription{C: s.ch, sub: s, cancel: func() {
		once.Do(func() {
			b.mu.Lock()
			if old, ok := b.subs[id]; ok {
				delete(b.subs, id)
				close(old.ch)
			}
			b.mu.Unlock()
		})
	}}
}
func (b *Bus) Publish(e Envelope) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return 0
	}
	dropped := 0
	for _, s := range b.subs {
		select {
		case s.ch <- e:
		default:
			s.dropped.Add(1)
			dropped++
		}
	}
	return dropped
}
func (b *Bus) RunSubscriber(ctx context.Context, buffer int, fn func(context.Context, Envelope)) {
	sub := b.Subscribe(buffer)
	defer sub.Cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-sub.C:
			if !ok {
				return
			}
			fn(ctx, e)
		}
	}
}
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for id, s := range b.subs {
		delete(b.subs, id)
		close(s.ch)
	}
}
