package engloop

import (
	"strings"
	"testing"
)

func TestEvaluateGremlinsBlocksCriticalLivedAndNotCovered(t *testing.T) {
	input := `{
		"test_efficacy": 91.2,
		"mutants_total": 4,
		"files": [
			{"file_name":"internal/authz/policy.go","mutations":[
				{"line":10,"column":2,"type":"CONDITIONALS_NEGATION","status":"LIVED"},
				{"line":11,"column":2,"type":"CONDITIONALS_BOUNDARY","status":"NOT COVERED"}
			]},
			{"file_name":"internal/cameras/model.go","mutations":[
				{"line":5,"column":1,"type":"ARITHMETIC_BASE","status":"LIVED"},
				{"line":6,"column":1,"type":"ARITHMETIC_BASE","status":"KILLED"}
			]}
		]
	}`
	report, err := EvaluateGremlins(strings.NewReader(input))
	if err != nil {
		t.Fatalf("EvaluateGremlins() error=%v", err)
	}
	if !report.HasCriticalBlockers() || len(report.CriticalBlockers) != 2 {
		t.Fatalf("critical blockers=%v", report.CriticalBlockers)
	}
	if len(report.NonCriticalLived) != 1 {
		t.Fatalf("noncritical lived=%v", report.NonCriticalLived)
	}
}

func TestEvaluateGremlinsKilledCriticalIsClean(t *testing.T) {
	input := `{"test_efficacy":100,"mutants_total":1,"files":[{"file_name":"internal/security/token.go","mutations":[{"line":1,"column":1,"type":"CONDITIONALS_NEGATION","status":"KILLED"}]}]}`
	report, err := EvaluateGremlins(strings.NewReader(input))
	if err != nil {
		t.Fatalf("EvaluateGremlins() error=%v", err)
	}
	if report.HasCriticalBlockers() {
		t.Fatalf("unexpected blockers=%v", report.CriticalBlockers)
	}
}
