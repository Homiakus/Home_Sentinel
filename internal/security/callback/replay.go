package callback

import (
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrReplay = errors.New("callback: replay detected")

type replayEntry struct {
	key       string
	expiresAt time.Time
}

// ReplayGuard is a bounded edge-level replay filter. ADGO SeenEvents remains
// the durable semantic deduplication boundary; this guard rejects repeated
// callback tokens before they reach the workflow service.
type ReplayGuard struct {
	mu      sync.Mutex
	max     int
	seen    map[string]time.Time
	ordered []replayEntry
}

func NewReplayGuard(maxEntries int) (*ReplayGuard, error) {
	if maxEntries <= 0 {
		return nil, errors.New("callback: replay guard capacity must be > 0")
	}
	return &ReplayGuard{max: maxEntries, seen: map[string]time.Time{}}, nil
}

func (g *ReplayGuard) Consume(claims Claims, now time.Time) error {
	if g == nil {
		return errors.New("callback: replay guard is nil")
	}
	if strings.TrimSpace(claims.KeyID) == "" || strings.TrimSpace(claims.Nonce) == "" {
		return ErrInvalidToken
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	expiresAt := time.Unix(claims.ExpiresAt, 0).UTC()
	if !expiresAt.After(now) {
		return ErrExpired
	}
	key := claims.KeyID + "\x00" + claims.Nonce

	g.mu.Lock()
	defer g.mu.Unlock()
	g.prune(now)
	if current, ok := g.seen[key]; ok && current.After(now) {
		return ErrReplay
	}
	g.seen[key] = expiresAt
	g.ordered = append(g.ordered, replayEntry{key: key, expiresAt: expiresAt})
	for len(g.ordered) > g.max {
		oldest := g.ordered[0]
		g.ordered = g.ordered[1:]
		if current, ok := g.seen[oldest.key]; ok && current.Equal(oldest.expiresAt) {
			delete(g.seen, oldest.key)
		}
	}
	return nil
}

func (g *ReplayGuard) prune(now time.Time) {
	cut := 0
	for cut < len(g.ordered) && !g.ordered[cut].expiresAt.After(now) {
		entry := g.ordered[cut]
		if current, ok := g.seen[entry.key]; ok && current.Equal(entry.expiresAt) {
			delete(g.seen, entry.key)
		}
		cut++
	}
	if cut > 0 {
		copy(g.ordered, g.ordered[cut:])
		g.ordered = g.ordered[:len(g.ordered)-cut]
	}
}
