package telemetry

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	defaultLogCapacity = 4096
	defaultMaxLogBytes = 8192
)

var (
	bearerPattern     = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]+`)
	secretPairPattern = regexp.MustCompile(`(?i)\b(token|password|passwd|secret|authorization|cookie|api[_-]?key|credential)\b(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`)
)

// LogSnapshot is an immutable view of the retained runtime log ring.
type LogSnapshot struct {
	Lines    []string
	Retained int
	Capacity int
}

// LogBuffer is a bounded, concurrency-safe io.Writer for newline-delimited logs.
// It stores sanitized lines only, so secrets are redacted before the UI can read them.
type LogBuffer struct {
	mu       sync.RWMutex
	lines    []string
	next     int
	full     bool
	pending  []byte
	maxBytes int
}

func NewLogBuffer(capacity, maxLineBytes int) *LogBuffer {
	if capacity <= 0 {
		capacity = defaultLogCapacity
	}
	if maxLineBytes <= 0 {
		maxLineBytes = defaultMaxLogBytes
	}
	return &LogBuffer{lines: make([]string, capacity), maxBytes: maxLineBytes}
}

func (b *LogBuffer) Write(p []byte) (int, error) {
	if b == nil || len(p) == 0 {
		return len(p), nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	b.pending = append(b.pending, p...)
	for {
		i := bytes.IndexByte(b.pending, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSuffix(string(b.pending[:i]), "\r")
		b.pending = b.pending[i+1:]
		b.storeLocked(line)
	}
	// Bound memory even if an external writer emits an unterminated line.
	if len(b.pending) > b.maxBytes*2 {
		b.storeLocked(string(b.pending[:b.maxBytes]) + " …[truncated]")
		b.pending = b.pending[:0]
	}
	return len(p), nil
}

func (b *LogBuffer) Snapshot(limit int) LogSnapshot {
	if b == nil {
		return LogSnapshot{}
	}
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := b.next
	if b.full {
		count = len(b.lines)
	}
	ordered := make([]string, 0, count)
	if b.full {
		ordered = append(ordered, b.lines[b.next:]...)
		ordered = append(ordered, b.lines[:b.next]...)
	} else {
		ordered = append(ordered, b.lines[:b.next]...)
	}
	if limit > 0 && len(ordered) > limit {
		ordered = append([]string(nil), ordered[len(ordered)-limit:]...)
	} else {
		ordered = append([]string(nil), ordered...)
	}
	return LogSnapshot{Lines: ordered, Retained: count, Capacity: len(b.lines)}
}

func (b *LogBuffer) storeLocked(line string) {
	line = strings.TrimSpace(line)
	if line == "" || len(b.lines) == 0 {
		return
	}
	line = sanitizeLogLine(line)
	if len(line) > b.maxBytes {
		cut := b.maxBytes
		if cut >= len(line) {
			cut = len(line)
		}
		for cut > 0 && cut < len(line) && !utf8.RuneStart(line[cut]) {
			cut--
		}
		line = line[:cut] + " …[truncated]"
	}
	b.lines[b.next] = line
	b.next++
	if b.next == len(b.lines) {
		b.next = 0
		b.full = true
	}
}

func sanitizeLogLine(line string) string {
	var value any
	if json.Unmarshal([]byte(line), &value) == nil {
		redactLogValue(value)
		if encoded, err := json.Marshal(value); err == nil {
			return string(encoded)
		}
	}
	return redactText(line)
}

func redactLogValue(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if sensitiveLogKey(key) {
				v[key] = "[REDACTED]"
				continue
			}
			switch childValue := child.(type) {
			case string:
				v[key] = redactText(childValue)
			default:
				redactLogValue(childValue)
			}
		}
	case []any:
		for i, child := range v {
			if text, ok := child.(string); ok {
				v[i] = redactText(text)
				continue
			}
			redactLogValue(child)
		}
	}
}

func sensitiveLogKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.NewReplacer("-", "_", ".", "_").Replace(key)
	if key == "key" {
		return true
	}
	for _, needle := range []string{"password", "passwd", "token", "secret", "authorization", "cookie", "credential", "api_key", "apikey", "private_key", "access_key"} {
		if strings.Contains(key, needle) {
			return true
		}
	}
	return false
}

func redactText(text string) string {
	text = bearerPattern.ReplaceAllString(text, "Bearer [REDACTED]")
	return secretPairPattern.ReplaceAllString(text, "$1$2[REDACTED]")
}
