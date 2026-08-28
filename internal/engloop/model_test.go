package engloop

import (
	"strings"
	"testing"
)

func TestClassifyPaths(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  RiskClass
	}{
		{name: "docs only", paths: []string{"docs/README.md"}, want: RiskLow},
		{name: "ordinary command", paths: []string{"cmd/sentinel/main.go"}, want: RiskMedium},
		{name: "api", paths: []string{"internal/api/server.go"}, want: RiskHigh},
		{name: "authz", paths: []string{"internal/authz/policy.go"}, want: RiskCritical},
		{name: "security config boundary", paths: []string{"internal/config/model.go"}, want: RiskCritical},
		{name: "orchestration incident", paths: []string{"internal/orchestration/incident/service.go"}, want: RiskCritical},
		{name: "engineering gate policy", paths: []string{"internal/engloop/model.go"}, want: RiskCritical},
		{name: "durable notifier", paths: []string{"internal/telegram/notifier.go"}, want: RiskCritical},
		{name: "highest wins", paths: []string{"internal/api/server.go", "internal/security/token.go"}, want: RiskCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyPaths(tt.paths); got != tt.want {
				t.Fatalf("ClassifyPaths()=%s want %s", got, tt.want)
			}
		})
	}
}

func TestGatePlanCritical(t *testing.T) {
	gates := GatePlan([]string{"internal/security/token.go"}, RiskCritical)
	for _, required := range []Gate{GateStatic, GateUnit, GateProperty, GateFuzz, GateRace, GateContract, GateFault, GateMutation, GateReplay, GateSecurity} {
		if !containsGate(gates, required) {
			t.Fatalf("critical gate plan missing %q: %v", required, gates)
		}
	}
}

func TestMutationTargetsCriticalProductionOnly(t *testing.T) {
	got := MutationTargets([]string{
		"internal/authz/policy.go",
		"internal/authz/policy_test.go",
		"internal/config/model.go",
		"internal/config/model_test.go",
		"internal/engloop/model.go",
		"internal/engloop/model_test.go",
		"internal/orchestration/incident/service.go",
		"internal/orchestration/incident/service_test.go",
		"internal/scenario/compiler/compiler.go",
		"internal/telegram/notifier.go",
		"internal/telegram/notifier_store.go",
		"internal/telegram/notifier_test.go",
		"internal/cameras/model.go",
	})
	want := []string{"./internal/authz", "./internal/config", "./internal/engloop", "./internal/orchestration/incident", "./internal/scenario/compiler", "./internal/telegram"}
	if len(got) != len(want) {
		t.Fatalf("MutationTargets()=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MutationTargets()[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestConfigSecurityBoundaryAlwaysGetsCriticalMutationGate(t *testing.T) {
	paths := []string{
		"internal/config/model.go",
		"internal/config/model_test.go",
	}
	if got := ClassifyPaths(paths); got != RiskCritical {
		t.Fatalf("ClassifyPaths()=%s want CRITICAL", got)
	}
	got := MutationTargets(paths)
	if len(got) != 1 || got[0] != "./internal/config" {
		t.Fatalf("MutationTargets()=%v want [./internal/config]", got)
	}
	gates := GatePlan(paths, ClassifyPaths(paths))
	if !containsGate(gates, GateMutation) {
		t.Fatalf("critical config gate plan missing mutation: %v", gates)
	}
	if !containsGate(gates, GateSecurity) {
		t.Fatalf("critical config gate plan missing security: %v", gates)
	}
}

func TestControlPlaneSafetyRootsAlwaysGetCriticalMutationGate(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		target string
	}{
		{name: "incident orchestration", path: "internal/orchestration/incident/service.go", target: "./internal/orchestration/incident"},
		{name: "siren orchestration", path: "internal/orchestration/action/siren/service.go", target: "./internal/orchestration/action/siren"},
		{name: "camera recovery", path: "internal/orchestration/recovery/camera/service.go", target: "./internal/orchestration/recovery/camera"},
		{name: "gate policy", path: "internal/engloop/model.go", target: "./internal/engloop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := []string{tt.path}
			if got := ClassifyPaths(paths); got != RiskCritical {
				t.Fatalf("ClassifyPaths(%q)=%s want CRITICAL", tt.path, got)
			}
			targets := MutationTargets(paths)
			if len(targets) != 1 || targets[0] != tt.target {
				t.Fatalf("MutationTargets(%q)=%v want [%s]", tt.path, targets, tt.target)
			}
			gates := GatePlan(paths, ClassifyPaths(paths))
			if !containsGate(gates, GateMutation) || !containsGate(gates, GateSecurity) {
				t.Fatalf("critical safety gate plan missing mutation/security: %v", gates)
			}
		})
	}
}

func TestMutationTargetsDurableNotifierCannotDisappearBehindMigration(t *testing.T) {
	paths := []string{
		"internal/database/migrations/0008_notification_delivery.sql",
		"internal/telegram/notifier.go",
		"internal/telegram/notifier_store.go",
		"internal/telegram/notifier_test.go",
	}
	if got := ClassifyPaths(paths); got != RiskCritical {
		t.Fatalf("ClassifyPaths()=%s want CRITICAL", got)
	}
	got := MutationTargets(paths)
	if len(got) != 1 || got[0] != "./internal/telegram" {
		t.Fatalf("MutationTargets()=%v want [./internal/telegram]", got)
	}
	gates := GatePlan(paths, ClassifyPaths(paths))
	if !containsGate(gates, GateMutation) {
		t.Fatalf("critical notifier gate plan missing mutation: %v", gates)
	}
}

func TestDecodeWorkPacketRejectsWeakCriticalPacket(t *testing.T) {
	input := `{
		"plan_item":"Stage-17.auth",
		"intent":"wire authorization",
		"status_before":"PARTIAL",
		"risk_class":"CRITICAL",
		"invariants":["system cannot unlock"],
		"code_surfaces":["internal/authz"],
		"required_gates":["static","unit","race"],
		"acceptance_evidence":["unauthorized request rejected"]
	}`
	_, err := DecodeWorkPacket(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "mutation-critical") {
		t.Fatalf("DecodeWorkPacket() error=%v, want mutation-critical requirement", err)
	}
}

func TestDecodeWorkPacketAcceptsCriticalPacket(t *testing.T) {
	input := `{
		"plan_item":"Stage-17.auth",
		"intent":"wire authorization",
		"status_before":"PARTIAL",
		"risk_class":"CRITICAL",
		"invariants":["system cannot unlock"],
		"contracts":["docs/THREAT_MODEL.md"],
		"code_surfaces":["internal/authz"],
		"required_gates":["static","unit","race","mutation-critical","security"],
		"acceptance_evidence":["unauthorized request rejected"]
	}`
	packet, err := DecodeWorkPacket(strings.NewReader(input))
	if err != nil {
		t.Fatalf("DecodeWorkPacket() error=%v", err)
	}
	if packet.PlanItem != "Stage-17.auth" {
		t.Fatalf("PlanItem=%q", packet.PlanItem)
	}
}

func containsGate(gates []Gate, want Gate) bool {
	for _, gate := range gates {
		if gate == want {
			return true
		}
	}
	return false
}
