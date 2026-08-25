//go:build go1.24

package homeassistant

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type coderWSRuntime struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

type wsMessage struct {
	ID      int      `json:"id,omitempty"`
	Type    string   `json:"type"`
	Success bool     `json:"success,omitempty"`
	Message string   `json:"message,omitempty"`
	Version string   `json:"ha_version,omitempty"`
	Event   *HAEvent `json:"event,omitempty"`
}

func newWSRuntime(parent context.Context, opts WSOptions, onState func(bool), onEvent func(StreamEvent)) (wsRuntime, error) {
	wsURL, err := websocketURL(opts.BaseURL)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	r := &coderWSRuntime{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(r.done)
		backoff := opts.ReconnectMin
		for ctx.Err() == nil {
			err := runHASession(ctx, wsURL, opts, onState, onEvent)
			onState(false)
			if ctx.Err() != nil {
				return
			}
			_ = err
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			backoff *= 2
			if backoff > opts.ReconnectMax {
				backoff = opts.ReconnectMax
			}
		}
	}()
	return r, nil
}

func runHASession(ctx context.Context, wsURL string, opts WSOptions, onState func(bool), onEvent func(StreamEvent)) error {
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.CloseNow()

	var msg wsMessage
	if err := wsjson.Read(ctx, conn, &msg); err != nil {
		return err
	}
	if msg.Type != "auth_required" {
		return fmt.Errorf("unexpected Home Assistant websocket pre-auth message %q", msg.Type)
	}
	if err := wsjson.Write(ctx, conn, map[string]any{"type": "auth", "access_token": opts.Token}); err != nil {
		return err
	}
	msg = wsMessage{}
	if err := wsjson.Read(ctx, conn, &msg); err != nil {
		return err
	}
	if msg.Type == "auth_invalid" {
		return fmt.Errorf("Home Assistant websocket authentication failed: %s", msg.Message)
	}
	if msg.Type != "auth_ok" {
		return fmt.Errorf("unexpected Home Assistant websocket auth result %q", msg.Type)
	}

	pending := make(map[int]struct{}, len(opts.EventTypes))
	nextID := 1
	for _, eventType := range opts.EventTypes {
		id := nextID
		nextID++
		pending[id] = struct{}{}
		if err := wsjson.Write(ctx, conn, map[string]any{"id": id, "type": "subscribe_events", "event_type": eventType}); err != nil {
			return err
		}
	}
	for len(pending) > 0 {
		msg = wsMessage{}
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			return err
		}
		switch msg.Type {
		case "result":
			if _, ok := pending[msg.ID]; ok {
				if !msg.Success {
					return fmt.Errorf("Home Assistant websocket subscription %d rejected", msg.ID)
				}
				delete(pending, msg.ID)
			}
		case "event":
			if msg.Event != nil {
				onEvent(StreamEvent{SubscriptionID: msg.ID, Event: *msg.Event})
			}
		}
	}
	onState(true)
	for {
		msg = wsMessage{}
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			return err
		}
		if msg.Type == "event" && msg.Event != nil {
			onEvent(StreamEvent{SubscriptionID: msg.ID, Event: *msg.Event})
		}
	}
}

func (r *coderWSRuntime) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() { r.cancel() })
	<-r.done
	return nil
}
