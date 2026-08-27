package engloop

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	goodSupplyMakefile = "GREMLINS_VERSION := v0.6.0\nGOVULNCHECK_VERSION := v1.7.0\nCYCLONEDX_GOMOD_VERSION := v1.10.0\n"
	goodSupplyPolicy   = "golang.org/x/vuln/cmd/govulncheck@v1.7.0\ngithub.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.10.0\nTrivy `v0.74.0`\n"
)

func TestVerifySupplyChainAcceptsImmutablePins(t *testing.T) {
	root := t.TempDir()
	writeSupplyChainBaseline(t, root, "steps:\n  - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1\n", goodSupplyMakefile, goodSupplyPolicy, "version: v0.74.0\n")

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
	writeSupplyChainBaseline(t, root, "steps:\n  - uses: actions/checkout@v7\n", goodSupplyMakefile, goodSupplyPolicy, "version: v0.74.0\n")

	report, err := VerifySupplyChain(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasBlockers() || !hasSupplyChainFinding(report.Findings, "action-ref-mutable") {
		t.Fatalf("mutable action ref was accepted: %+v", report.Findings)
	}
}

func TestVerifySupplyChainRejectsFloatingGoInstall(t *testing.T) {
	root := t.TempDir()
	workflow := "steps:\n  - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1\n  - run: go install example.com/tool@" + "latest\n"
	writeSupplyChainBaseline(t, root, workflow, goodSupplyMakefile, goodSupplyPolicy, "version: v0.74.0\n")

	report, err := VerifySupplyChain(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasBlockers() || !hasSupplyChainFinding(report.Findings, "floating-go-tool") {
		t.Fatalf("floating go install was accepted: %+v", report.Findings)
	}
}

func TestVerifySupplyChainRejectsPinDriftAndMissingAutomation(t *testing.T) {
	root := t.TempDir()
	writeSupplyChainFixture(t, root, ".github/workflows/ci.yml", "steps:\n  - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1\n")
	writeSupplyChainFixture(t, root, ".github/workflows/security.yml", "version: v0.72.0\n")
	writeSupplyChainFixture(t, root, "Makefile", "GREMLINS_VERSION := v0.5.0\nGOVULNCHECK_VERSION := v1.1.4\nCYCLONEDX_GOMOD_VERSION := v1.9.0\n")
	writeSupplyChainFixture(t, root, "docs/security/SUPPLY_CHAIN.md", "golang.org/x/vuln/cmd/govulncheck@v1.1.4\ngithub.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.9.0\nTrivy `v0.72.0`\n")

	report, err := VerifySupplyChain(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasBlockers() {
		t.Fatal("tool pin drift was accepted")
	}
	for _, id := range []string{
		"gremlins-pin-drift",
		"govulncheck-pin-drift",
		"cyclonedx-pin-drift",
		"govulncheck-policy-drift",
		"cyclonedx-policy-drift",
		"trivy-policy-drift",
		"trivy-workflow-pin-drift",
		"dependabot-missing",
	} {
		if !hasSupplyChainFinding(report.Findings, id) {
			t.Fatalf("missing %s: %+v", id, report.Findings)
		}
	}
}

func writeSupplyChainBaseline(t *testing.T, root, ci, makefile, policy, security string) {
	t.Helper()
	writeSupplyChainFixture(t, root, ".github/workflows/ci.yml", ci)
	writeSupplyChainFixture(t, root, ".github/workflows/security.yml", security)
	writeSupplyChainFixture(t, root, ".github/dependabot.yml", "version: 2\nupdates: []\n")
	writeSupplyChainFixture(t, root, "Makefile", makefile)
	writeSupplyChainFixture(t, root, "docs/security/SUPPLY_CHAIN.md", policy)
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
