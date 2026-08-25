package frigate

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeControl struct {
	cfg             map[string]any
	saveN, restartN int
	failRestartOnce bool
	streams         map[string]json.RawMessage
}

func (f *fakeControl) Version(context.Context) (string, error)        { return "0.16", nil }
func (f *fakeControl) Config(context.Context) (map[string]any, error) { return f.cfg, nil }
func (f *fakeControl) SaveConfig(_ context.Context, b []byte, _ bool) error {
	f.saveN++
	var x map[string]any
	if err := json.Unmarshal(b, &x); err != nil {
		return err
	}
	f.cfg = x
	return nil
}
func (f *fakeControl) Restart(context.Context) error {
	f.restartN++
	if f.failRestartOnce {
		f.failRestartOnce = false
		return errors.New("boom")
	}
	return nil
}
func (f *fakeControl) Go2RTCStreams(context.Context) (map[string]json.RawMessage, error) {
	return f.streams, nil
}

type memorySink struct{ called bool }

func (m *memorySink) Materialize(map[string]string) error { m.called = true; return nil }
func TestApplySuccess(t *testing.T) {
	f := &fakeControl{cfg: map[string]any{"old": true}, streams: map[string]json.RawMessage{"cam_x": json.RawMessage(`{}`)}}
	s := &memorySink{}
	r, err := (Applier{Control: f, Secrets: s, PollInterval: time.Millisecond}).Apply(context.Background(), ApplyRequest{ConfigJSON: []byte(`{"cameras":{},"go2rtc":{}}`), SecretEnv: map[string]string{"FRIGATE_X": "x"}, ExpectedStreams: []string{"cam_x"}, ReadyTimeout: time.Second})
	if err != nil || !r.Applied || !s.called {
		t.Fatalf("r=%+v err=%v", r, err)
	}
}
func TestApplyRollsBackOnRestartFailure(t *testing.T) {
	f := &fakeControl{cfg: map[string]any{"old": true}, streams: map[string]json.RawMessage{}, failRestartOnce: true}
	r, err := (Applier{Control: f, PollInterval: time.Millisecond}).Apply(context.Background(), ApplyRequest{ConfigJSON: []byte(`{"new":true}`), ReadyTimeout: time.Second})
	if err == nil || !r.RolledBack {
		t.Fatalf("r=%+v err=%v", r, err)
	}
	if _, ok := f.cfg["old"]; !ok {
		t.Fatalf("rollback cfg=%v", f.cfg)
	}
}
