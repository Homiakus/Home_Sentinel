package app

import (
	"errors"
	"testing"
)

func TestAppCloseOwnsIncidentRuntimeAndReturnsItsFailure(t *testing.T) {
	closeBoom := errors.New("incident close boom")
	workflow := &appIncidentWorkflowFake{closeErr: closeBoom}
	done := make(chan struct{})
	close(done)
	cancelCalls := 0
	runtime := &IncidentRuntime{
		Workflow: workflow,
		cancel:   func() { cancelCalls++ },
		done:     done,
	}
	a := &App{IncidentRuntime: runtime}

	if err := a.Close(); !errors.Is(err, closeBoom) {
		t.Fatalf("App.Close error=%v want=%v", err, closeBoom)
	}
	if a.IncidentRuntime != nil {
		t.Fatal("App.Close retained incident runtime after shutdown")
	}
	if cancelCalls != 1 || workflow.closeCalls != 1 {
		t.Fatalf("shutdown calls cancel=%d workflow=%d", cancelCalls, workflow.closeCalls)
	}
}
