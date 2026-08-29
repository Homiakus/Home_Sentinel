package topology

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Homiakus/axiom/adgo"
)

const writerLockDirectory = ".control-plane-writer"

var (
	ErrRuntimeRootRequired = errors.New("topology: runtime root is required")
	ErrWriterUnavailable   = errors.New("topology: control-plane writer is unavailable")
)

// WriterGuard enforces the supported v1 topology: exactly one Home Sentinel
// control-plane writer owns one canonical runtime root for the process lifetime.
// Pebble owns the underlying cross-process filesystem lock until Close.
type WriterGuard struct {
	mu    sync.Mutex
	root  string
	store *adgo.PebbleStore
}

// CanonicalRoot resolves aliases before writer ownership is derived. The root
// is created first so EvalSymlinks can safely resolve an installation path that
// did not exist before first startup.
func CanonicalRoot(runtimeRoot string) (string, error) {
	root := strings.TrimSpace(runtimeRoot)
	if root == "" {
		return "", ErrRuntimeRootRequired
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("topology: absolute runtime root: %w", err)
	}
	abs = filepath.Clean(abs)
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return "", fmt.Errorf("topology: create runtime root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("topology: resolve runtime root: %w", err)
	}
	return filepath.Clean(resolved), nil
}

// AcquireWriter obtains process-lifetime ownership for runtimeRoot. Any failure
// to open the dedicated Pebble lock DB is fail-closed: callers must not continue
// application startup in a degraded multi-writer mode.
func AcquireWriter(runtimeRoot string) (*WriterGuard, error) {
	root, err := CanonicalRoot(runtimeRoot)
	if err != nil {
		return nil, err
	}
	store, err := adgo.OpenPebbleStore(filepath.Join(root, writerLockDirectory))
	if err != nil {
		return nil, fmt.Errorf("%w: root=%q: %v", ErrWriterUnavailable, root, err)
	}
	return &WriterGuard{root: root, store: store}, nil
}

func (g *WriterGuard) Root() string {
	if g == nil {
		return ""
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.root
}

// Close releases writer ownership. It is intentionally idempotent so partial
// startup cleanup and normal App.Close can share one cleanup path safely.
func (g *WriterGuard) Close() error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.store == nil {
		return nil
	}
	store := g.store
	g.store = nil
	return store.Close()
}
