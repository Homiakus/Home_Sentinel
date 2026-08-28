package app

import (
	"context"
	"errors"
	"testing"
	"time"

	orchestrationincident "github.com/Homiakus/Home_Sentinel/internal/orchestration/incident"
)

func TestClassifyIncidentServeExit(t *testing.T) {
	boom := errors.New("serve failed")
	cases := []struct {
		name         string
		lifecycleErr error
		serveErr     error
		want         error
		wantNil      bool
	}{
		{name: "normal cancellation", lifecycleErr: context.Canceled, serveErr: context.Canceled, wantNil: true},
		{name: "normal deadline cancellation", lifecycleErr: context.DeadlineExceeded, serveErr: boom, wantNil: true},
		{name: "unexpected explicit failure", serveErr: boom, want: boom},
		{name: "unexpected nil exit", serveErr: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyIncidentServeExit(tc.lifecycleErr, tc.serveErr)
			if tc.wantNil {
				if err != nil {
					t.Fatalf("error=%v want nil", err)
				}
				return
			}
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("error=%v want %v", err, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatal("unexpected nil Serve exit was accepted")
			}
		})
	}
}

func TestIncidentRuntimeParentCancellationFencesNewOperations(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	runtime, err := openIncidentRuntime(parent, incidentRuntimeOptions{
		Config:   memoryIncidentConfig(),
		Notifier: &appIncidentNotifierFake{},
	})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-runtime.done:
	case <-time.After(time.Second):
		t.Fatal("incident Serve loop did not stop after parent cancellation")
	}
	if _, err := runtime.Start(context.Background(), validAppIncidentTrigger()); !errors.Is(err, ErrIncidentRuntimeUnavailable) {
		t.Fatalf("start after lifecycle cancellation error=%v", err)
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("close after parent cancellation: %v", err)
	}
}

func TestIncidentRuntimeOperationalRejectsStoppedAndClosedStates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		stopped bool
		closed  bool
	}{
		{name: "stopped", stopped: true},
		{name: "closed", closed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &IncidentRuntime{Workflow: &orchestrationincident.Service{}, stopped: tc.stopped, closed: tc.closed}
			if err := runtime.operational(); !errors.Is(err, ErrIncidentRuntimeUnavailable) {
				t.Fatalf("operational error=%v", err)
			}
		})
	}
}
