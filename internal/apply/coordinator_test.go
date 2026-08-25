package apply

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fake struct {
	fail  string
	calls []string
}

func (f *fake) add(s string) { f.calls = append(f.calls, s) }
func (f *fake) Plan(context.Context, []byte) (Plan, error) {
	f.add("plan")
	return Plan{Changed: true}, nil
}
func (f *fake) Validate(context.Context, []byte, Plan) error {
	f.add("validate")
	if f.fail == "validate" {
		return errors.New("x")
	}
	return nil
}
func (f *fake) Backup(context.Context, Plan) (Backup, error) {
	f.add("backup")
	return Backup{ID: "b", CreatedAt: time.Now()}, nil
}
func (f *fake) Apply(context.Context, []byte, Plan) error {
	f.add("apply")
	if f.fail == "apply" {
		return errors.New("x")
	}
	return nil
}
func (f *fake) Verify(context.Context, []byte, Plan) error {
	f.add("verify")
	if f.fail == "verify" {
		return errors.New("x")
	}
	return nil
}
func (f *fake) Commit(context.Context, []byte, Plan) error {
	f.add("commit")
	if f.fail == "commit" {
		return errors.New("x")
	}
	return nil
}
func (f *fake) Rollback(context.Context, Backup, error) error { f.add("rollback"); return nil }
func TestCoordinatorRollback(t *testing.T) {
	f := &fake{fail: "verify"}
	r, err := (Coordinator{}).Run(context.Background(), f, []byte("x"))
	if err == nil {
		t.Fatal("expected failure")
	}
	if !r.Applied || !r.RolledBack {
		t.Fatalf("result=%+v calls=%v", r, f.calls)
	}
	if f.calls[len(f.calls)-1] != "rollback" {
		t.Fatalf("calls=%v", f.calls)
	}
}
func TestCoordinatorSuccess(t *testing.T) {
	f := &fake{}
	r, err := (Coordinator{}).Run(context.Background(), f, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Applied || r.RolledBack {
		t.Fatalf("result=%+v", r)
	}
}
