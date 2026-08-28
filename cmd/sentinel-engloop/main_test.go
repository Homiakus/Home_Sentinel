package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunActivePacketTextAndJSON(t *testing.T) {
	root := activePacketCLIFixture(t)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "text", args: []string{"active-packet", "--root", root}, want: "ACTIVE Stage 17 CLI fixture risk=CRITICAL base=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa max_commits=8"},
		{name: "json", args: []string{"active-packet", "--root", root, "--json"}, want: `"mutation_base": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := run(tc.args, &stdout, &stderr); err != nil {
				t.Fatalf("run error=%v stderr=%s", err, stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.want) {
				t.Fatalf("stdout=%q want substring %q", stdout.String(), tc.want)
			}
		})
	}
}

func TestRunActivePacketRejectsMissingDescriptor(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"active-packet", "--root", t.TempDir()}, &stdout, &stderr)
	if err == nil {
		t.Fatal("missing active descriptor accepted")
	}
	if !strings.Contains(err.Error(), "open active work packet") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunMutationRejectsZeroMutantEvidence(t *testing.T) {
	path := writeMutationFixture(t, `{"test_efficacy":0,"mutants_total":0,"files":[]}`)
	var stdout, stderr bytes.Buffer
	err := run([]string{"mutation", "--file", path}, &stdout, &stderr)
	var exitErr exitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("zero-mutant report error=%v, want exitError", err)
	}
	if exitErr.code != 11 {
		t.Fatalf("zero-mutant exit code=%d want 11; error=%v", exitErr.code, err)
	}
	if !strings.Contains(exitErr.msg, "zero generated mutants") {
		t.Fatalf("zero-mutant error=%q", exitErr.msg)
	}
	if !strings.Contains(stdout.String(), "total=0") {
		t.Fatalf("zero-mutant diagnostic missing total: %q", stdout.String())
	}
}

func TestRunMutationAcceptsKilledCriticalEvidence(t *testing.T) {
	path := writeMutationFixture(t, `{"test_efficacy":100,"mutants_total":1,"files":[{"file_name":"internal/engloop/mutation.go","mutations":[{"line":1,"column":1,"type":"CONDITIONALS_BOUNDARY","status":"KILLED"}]}]}`)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"mutation", "--file", path}, &stdout, &stderr); err != nil {
		t.Fatalf("killed mutation evidence rejected: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "total=1") || !strings.Contains(stdout.String(), "critical_blockers=0") {
		t.Fatalf("unexpected killed mutation diagnostic: %q", stdout.String())
	}
}

func writeMutationFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gremlins.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func activePacketCLIFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	packet := `{
  "plan_item":"Stage 17 CLI fixture",
  "intent":"validate active packet CLI",
  "status_before":"PARTIAL",
  "risk_class":"CRITICAL",
  "invariants":["packet is exact"],
  "code_surfaces":["internal/security/callback/accept.go"],
  "required_gates":["static","unit","race","mutation-critical"],
  "acceptance_evidence":["CLI validates descriptor"]
}`
	active := `{
  "packet":"docs/engineering/work-packets/fixture.json",
  "mutation_base":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "max_commits":8
}`
	for rel, content := range map[string]string{
		"docs/engineering/work-packets/fixture.json": packet,
		"docs/engineering/ACTIVE_WORK_PACKET.json":   active,
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
