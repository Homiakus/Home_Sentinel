package engloop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySupplyChainAcceptsImmutablePins(t *testing.T) {
	root := t.TempDir()
	writeSupplyChainFixture(t, root, ".github/workflows/ci.yml", "steps:\n  - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1\n")
	writeSupplyChainFixture(t, root, "Makefile", "GREMLINS_VERSION := v0.6.0\nGOVULNCHECK_VERSION := v1.7.0\n")
	writeSupplyChainFixture(t, root, "docs/security/SUPPLY_CHAIN.md", "golang.org/x/vuln/cmd/govulncheck@v1.7.0\n")

	report, err := VerifySupplyChain(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.HasBlockers() {
		t.Fatalf("unexpected blockers: %+v", report.Findings)
	}
}

func TestVerifySupplyChainRejectsMutableActionRef(t *testing.T) {
	root := t.TempDir()
	writeSupplyChainFixture(t, root, ".github/workflows/ci.yml", "steps:\n  - uses: actions/checkout@v7\n")
	writeSupplyChainFixture(t, root, "Makefile", "GREMLINS_VERSION := v0.6.0\nGOVULNCHECK_VERSION := v1.7.0\n")
	writeSupplyChainFixture(t, root, "docs/security/SUPPLY_CHAIN.md", "golang.org/x/vuln/cmd/govulncheck@v1.7.0\n")

	report, err := VerifySupplyChain(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasBlockers() {
		t.Fatal("mutable action ref was accepted")
	}
	if !hasSupplyChainFinding(report.Findings, "action-ref-mutable") {
		t.Fatalf("missing mutable-action finding: %+v", report.Findings)
	}
}

func TestVerifySupplyChainRejectsFloatingGoInstall(t *testing.T) {
	root := t.TempDir()
	writeSupplyChainFixture(t, root, ".github/workflows/ci.yml", "steps:\n  - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1\n  - run: go install example.com/tool@latest\n")
	writeSupplyChainFixture(t, root, "Makefile", "GREMLINS_VERSION := v0.6.0\nGOVULNCHECK_VERSION := v1.7.0\n")
	writeSupplyChainFixture(t, root, "docs/security/SUPPLY_CHAIN.md", "golang.org/x/vuln/cmd/govulncheck@v1.7.0\n")

	report, err := VerifySupplyChain(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasBlockers() || !hasSupplyChainFinding(report.Findings, "floating-go-tool") {
		t.Fatalf("floating go install was accepted: %+v", report.Findings)
	}
}

func TestVerifySupplyChainRejectsPinDrift(t *testing.T) {
	root := t.TempDir()
	writeSupplyChainFixture(t, root, ".github/workflows/ci.yml", "steps:\n  - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1\n")
	writeSupplyChainFixture(t, root, "Makefile", "GREMLINS_VERSION := v0.5.0\nGOVULNCHECK_VERSION := v1.1.4\n")
	writeSupplyChainFixture(t, root, "docs/security/SUPPLY_CHAIN.md", "golang.org/x/vuln/cmd/govulncheck@v1.1.4\n")

	report, err := VerifySupplyChain(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasBlockers() {
		t.Fatal("tool pin drift was accepted")
	}
	for _, id := range []string{"gremlins-pin-drift", "govulncheck-pin-drift", "govulncheck-policy-drift"} {
		if !hasSupplyChainFinding(report.Findings, id) {
			t.Fatalf("missing %s: %+v", id, report.Findings)
		}
	}
}

func writeSupplyChainFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func hasSupplyChainFinding(findings []Finding, id string) bool {
	for _, finding := range findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}
