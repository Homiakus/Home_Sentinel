package engloop

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	goodSupplyMakefile = "GREMLINS_VERSION := v0.6.0\nGOVULNCHECK_VERSION := v1.7.0\nCYCLONEDX_GOMOD_VERSION := v1.10.0\n"
	goodSupplyPolicy   = "golang.org/x/vuln/cmd/govulncheck@v1.7.0\ngithub.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.10.0\nTrivy `v0.74.0`\nCI_ACTION_PINS.json\n"
	goodActionPins     = `{
  "schema_version": 1,
  "actions": {
    "actions/checkout": {"version": "v7.0.1", "sha": "3d3c42e5aac5ba805825da76410c181273ba90b1"},
    "actions/setup-go": {"version": "v7.0.0", "sha": "b7ad1dad31e06c5925ef5d2fc7ad053ef454303e"},
    "actions/upload-artifact": {"version": "v7.0.1", "sha": "043fb46d1a93c77aae656e7c1c64a875d1fc6a0a"},
    "aquasecurity/trivy-action": {"version": "v0.33.1", "sha": "ed142fd0673e97e23eac54620cfb913e5ce36c25"}
  }
}`
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

func TestVerifySupplyChainRejectsUnreviewedImmutableSHA(t *testing.T) {
	root := t.TempDir()
	writeSupplyChainBaseline(t, root, "steps:\n  - uses: actions/checkout@1111111111111111111111111111111111111111\n", goodSupplyMakefile, goodSupplyPolicy, "version: v0.74.0\n")

	report, err := VerifySupplyChain(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasBlockers() || !hasSupplyChainFinding(report.Findings, "action-pin-drift") {
		t.Fatalf("unreviewed immutable SHA was accepted: %+v", report.Findings)
	}
}

func TestVerifySupplyChainRejectsUnknownAction(t *testing.T) {
	root := t.TempDir()
	workflow := "steps:\n  - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1\n  - uses: example/unknown-action@2222222222222222222222222222222222222222\n"
	writeSupplyChainBaseline(t, root, workflow, goodSupplyMakefile, goodSupplyPolicy, "version: v0.74.0\n")

	report, err := VerifySupplyChain(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasBlockers() || !hasSupplyChainFinding(report.Findings, "action-not-reviewed") {
		t.Fatalf("unknown action was accepted: %+v", report.Findings)
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
	writeSupplyChainFixture(t, root, "docs/security/SUPPLY_CHAIN.md", "golang.org/x/vuln/cmd/govulncheck@v1.1.4\ngithub.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.9.0\nTrivy `v0.72.0`\nCI_ACTION_PINS.json\n")
	writeSupplyChainFixture(t, root, "docs/security/CI_ACTION_PINS.json", goodActionPins)

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
	writeSupplyChainFixture(t, root, "docs/security/CI_ACTION_PINS.json", goodActionPins)
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
