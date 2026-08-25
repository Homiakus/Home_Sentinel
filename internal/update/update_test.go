package update

import (
	"context"
	"errors"
	"os"
	"testing"
)

type cpFake struct{ created, restored bool }

func (c *cpFake) Create(context.Context) (string, error) { c.created = true; return "cp1", nil }
func (c *cpFake) Restore(context.Context, string) error  { c.restored = true; return nil }

type exFake struct {
	fail     string
	rollback bool
}

func (e *exFake) Stage(context.Context, Plan) error {
	if e.fail == "stage" {
		return errors.New("x")
	}
	return nil
}
func (e *exFake) Activate(context.Context, Plan) error {
	if e.fail == "activate" {
		return errors.New("x")
	}
	return nil
}
func (e *exFake) Verify(context.Context, Plan) error {
	if e.fail == "verify" {
		return errors.New("x")
	}
	return nil
}
func (e *exFake) Rollback(context.Context, Plan) error { e.rollback = true; return nil }
func TestPlanRejectsLatest(t *testing.T) {
	m := Manifest{Version: "1.1.0", SchemaTarget: 7, MinReadableSchema: 7, Components: map[string]Component{"frigate": {Ref: "ghcr.io/x/frigate:latest"}}}
	if m.Validate() == nil {
		t.Fatal("expected latest rejection")
	}
}
func TestPlanCompatibilityAndActions(t *testing.T) {
	m := Manifest{Version: "1.1.0", MinCurrentVersion: "1.0.0", SchemaTarget: 8, MinReadableSchema: 7, Components: map[string]Component{"frigate": {Ref: "ghcr.io/x/frigate:0.17.0"}}}
	p, e := BuildPlan(Current{Version: "1.0.0", SchemaVersion: 7, Components: map[string]string{"frigate": "ghcr.io/x/frigate:0.16.0"}}, m)
	if e != nil || !p.Compatible || len(p.Actions) != 1 || !p.Actions[0].Changed {
		t.Fatalf("bad plan %+v %v", p, e)
	}
}
func TestManagerRollbackOnVerifyFailure(t *testing.T) {
	cp := &cpFake{}
	ex := &exFake{fail: "verify"}
	r, e := (Manager{Checkpoint: cp, Executor: ex}).Apply(context.Background(), Plan{Compatible: true})
	if e == nil || !r.RolledBack || !cp.restored || !ex.rollback {
		t.Fatalf("rollback missing %+v %v", r, e)
	}
}
func TestIrreversibleMigrationRequiresRestore(t *testing.T) {
	d := DecideMigration(7, 8, 7, true)
	if !d.Allowed || !d.RestoreRequiredForRollback {
		t.Fatalf("bad decision %+v", d)
	}
}

func TestExampleReleaseManifestParses(t *testing.T) {
	b, err := os.ReadFile("../../deploy/release/release-manifest.example.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseManifest(b); err != nil {
		t.Fatal(err)
	}
}
