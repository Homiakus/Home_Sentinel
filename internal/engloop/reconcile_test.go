package engloop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReconcileDetectsStaleGoSumAndUnwiredCI(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "go.sum", "example h1:abc\n")
	mustWrite(t, root, "docs/PLAN_INDEX.md", "engineering/ENGINEERING_LOOP.md\n")
	mustWrite(t, root, "docs/engineering/ENGINEERING_LOOP.md", "loop\n")
	mustWrite(t, root, "docs/testing/EDGE_SPACE_MODEL.md", "edge\n")
	mustWrite(t, root, "docs/testing/MUTATION_TESTING.md", "Gremlins\n")
	mustWrite(t, root, "docs/testing/STRATEGY.md", "mesh\n")
	mustWrite(t, root, "docs/AXIOM_IMPLEMENTATION_PLAN.md", "master\n")
	mustWrite(t, root, "docs/SCENARIO_SYSTEM_PLAN.md", "scenario\n")
	mustWrite(t, root, "docs/IMPLEMENTATION_STATUS.md", "repository still has no committed `go.sum`\n")
	mustWrite(t, root, ".github/workflows/ci.yml", "cache: false # go.sum is not committed yet\n")

	report, err := Reconcile(root)
	if err != nil {
		t.Fatalf("Reconcile() error=%v", err)
	}
	for _, id := range []string{"status-go-sum-stale", "ci-module-lock-comment-stale", "ci-engloop-not-wired", "mutation-engine-unpinned"} {
		if !hasFinding(report, id) {
			t.Fatalf("missing finding %q in %#v", id, report.Findings)
		}
	}
	if report.HasBlockers() {
		t.Fatalf("warnings should not be blockers: %#v", report.Findings)
	}
}

func TestReconcileMissingProtocolIsBlocker(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "docs/PLAN_INDEX.md", "no loop binding\n")

	report, err := Reconcile(root)
	if err != nil {
		t.Fatalf("Reconcile() error=%v", err)
	}
	if !report.HasBlockers() {
		t.Fatalf("expected blockers: %#v", report.Findings)
	}
	if !hasFinding(report, "plan-index-loop-unbound") {
		t.Fatalf("missing plan-index-loop-unbound: %#v", report.Findings)
	}
}

func hasFinding(report ReconcileReport, id string) bool {
	for _, finding := range report.Findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}

func mustWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", rel, err)
	}
}
