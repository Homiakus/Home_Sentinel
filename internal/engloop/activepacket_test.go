package engloop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadActiveWorkPacketValidatesReferencedPacket(t *testing.T) {
	root := t.TempDir()
	packetRel := "docs/engineering/work-packets/stage17.json"
	writeActiveFixture(t, root, packetRel, `{
  "plan_item":"Stage 17 callback ingress",
  "intent":"prove exact callback acceptance",
  "status_before":"PARTIAL",
  "risk_class":"CRITICAL",
  "invariants":["exact binding"],
  "code_surfaces":["internal/security/callback/accept.go"],
  "required_gates":["static","unit","race","mutation-critical"],
  "acceptance_evidence":["mutation clean"]
}`)
	writeActiveFixture(t, root, ActiveWorkPacketPath, `{
  "packet":"docs/engineering/work-packets/stage17.json",
  "mutation_base":"38163bba63b7dee147bd1edfe579db86c11a53f7",
  "max_commits":64
}`)
	active, packet, err := LoadActiveWorkPacket(root)
	if err != nil {
		t.Fatal(err)
	}
	if active.MaxCommits != 64 || packet.RiskClass != RiskCritical || packet.PlanItem != "Stage 17 callback ingress" {
		t.Fatalf("active=%+v packet=%+v", active, packet)
	}
}

func TestActiveWorkPacketAcceptsInclusiveCommitBoundaries(t *testing.T) {
	for _, max := range []int{1, 256} {
		active := ActiveWorkPacket{
			Packet:       "docs/engineering/work-packets/p.json",
			MutationBase: strings.Repeat("a", 40),
			MaxCommits:   max,
		}
		if err := active.Validate(); err != nil {
			t.Fatalf("max_commits=%d rejected: %v", max, err)
		}
	}
}

func TestActiveWorkPacketRejectsTraversalAndMutableBase(t *testing.T) {
	cases := []ActiveWorkPacket{
		{Packet: "../packet.json", MutationBase: strings.Repeat("a", 40), MaxCommits: 10},
		{Packet: "docs/engineering/work-packets/p.json", MutationBase: "HEAD^", MaxCommits: 10},
		{Packet: "docs/engineering/work-packets/p.json", MutationBase: strings.Repeat("a", 40), MaxCommits: 0},
		{Packet: "docs/engineering/work-packets/p.json", MutationBase: strings.Repeat("a", 40), MaxCommits: 257},
	}
	for i, tc := range cases {
		if err := tc.Validate(); err == nil {
			t.Fatalf("case %d accepted: %+v", i, tc)
		}
	}
}

func TestLoadActiveWorkPacketRejectsReferencedPacketOutsideRoot(t *testing.T) {
	root := t.TempDir()
	writeActiveFixture(t, root, ActiveWorkPacketPath, `{
  "packet":"docs/engineering/work-packets/../escape.json",
  "mutation_base":"38163bba63b7dee147bd1edfe579db86c11a53f7",
  "max_commits":64
}`)
	if _, _, err := LoadActiveWorkPacket(root); err == nil {
		t.Fatal("path traversal packet accepted")
	}
}

func TestLoadActiveWorkPacketRejectsMultipleJSONValues(t *testing.T) {
	root := t.TempDir()
	writeActiveFixture(t, root, ActiveWorkPacketPath, `{
  "packet":"docs/engineering/work-packets/p.json",
  "mutation_base":"38163bba63b7dee147bd1edfe579db86c11a53f7",
  "max_commits":64
} {"extra":true}`)
	_, _, err := LoadActiveWorkPacket(root)
	if err == nil {
		t.Fatal("multiple active-packet JSON values accepted")
	}
	if !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("multiple JSON values produced wrong diagnostic: %v", err)
	}
}

func TestLoadActiveWorkPacketRejectsInvalidReferencedPacket(t *testing.T) {
	root := t.TempDir()
	packetRel := "docs/engineering/work-packets/bad.json"
	writeActiveFixture(t, root, packetRel, `{"plan_item":"missing everything else"}`)
	writeActiveFixture(t, root, ActiveWorkPacketPath, `{
  "packet":"docs/engineering/work-packets/bad.json",
  "mutation_base":"38163bba63b7dee147bd1edfe579db86c11a53f7",
  "max_commits":64
}`)
	if _, _, err := LoadActiveWorkPacket(root); err == nil {
		t.Fatal("invalid referenced work packet accepted")
	}
}

func writeActiveFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
