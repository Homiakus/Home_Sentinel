package resourceguard

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Homiakus/axiom/adgo"
)

type testRequest struct {
	Resource string `json:"resource"`
}

func testResolver(execution *adgo.Execution) (string, error) {
	var request testRequest
	if err := json.Unmarshal(execution.Data["request"], &request); err != nil {
		return "", err
	}
	return request.Resource, nil
}

func createExecution(t *testing.T, store *adgo.MemoryStore, id, resource string, status adgo.ExecutionStatus) {
	t.Helper()
	raw, err := json.Marshal(testRequest{Resource: resource})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(context.Background(), &adgo.Execution{
		ID: id, PlanID: "plan", PlanVersion: "1", PlanDigest: "digest", Version: 1,
		Status: status, Data: map[string]json.RawMessage{"request": raw},
	}); err != nil {
		t.Fatalf("create execution: %v", err)
	}
}

func TestCheckBlocksNonTerminalOwnerAndReleasesTerminal(t *testing.T) {
	ctx := context.Background()
	store := adgo.NewMemoryStore()
	createExecution(t, store, "owner", "door:front", adgo.StatusHuman)

	err := Check(ctx, store, "plan", "candidate", "door:front", testResolver)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("expected busy resource, got %v", err)
	}
	if err := Check(ctx, store, "plan", "candidate", "door:rear", testResolver); err != nil {
		t.Fatalf("different resource blocked: %v", err)
	}
	if err := Check(ctx, store, "plan", "owner", "door:front", testResolver); err != nil {
		t.Fatalf("idempotent same execution blocked: %v", err)
	}

	current, err := store.Load(ctx, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Commit(ctx, "owner", current.Version, func(next *adgo.Execution) error {
		next.Status = adgo.StatusCompleted
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := Check(ctx, store, "plan", "candidate", "door:front", testResolver); err != nil {
		t.Fatalf("terminal owner did not release resource: %v", err)
	}
}

func TestCheckFailsClosedOnCorruptOwnerState(t *testing.T) {
	ctx := context.Background()
	store := adgo.NewMemoryStore()
	if err := store.Create(ctx, &adgo.Execution{
		ID: "corrupt", PlanID: "plan", PlanVersion: "1", PlanDigest: "digest", Version: 1,
		Status: adgo.StatusWaiting, Data: map[string]json.RawMessage{"request": json.RawMessage("{")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := Check(ctx, store, "plan", "candidate", "door:front", testResolver); err == nil || errors.Is(err, ErrBusy) {
		t.Fatalf("expected fail-closed decode error, got %v", err)
	}
}
