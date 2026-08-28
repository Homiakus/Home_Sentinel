package engloop

import (
	"reflect"
	"testing"
)

func TestIncidentRuntimeIsCriticalMutationSurface(t *testing.T) {
	path := "internal/app/incident_runtime.go"
	if got := ClassifyPaths([]string{path}); got != RiskCritical {
		t.Fatalf("risk=%s want=%s", got, RiskCritical)
	}
	want := []string{"./internal/app"}
	if got := MutationTargets([]string{path}); !reflect.DeepEqual(got, want) {
		t.Fatalf("mutation targets=%v want=%v", got, want)
	}
}

func TestUnrelatedApplicationFileDoesNotBecomeCritical(t *testing.T) {
	path := "internal/app/app.go"
	if got := ClassifyPaths([]string{path}); got == RiskCritical {
		t.Fatalf("generic app bootstrap unexpectedly classified critical")
	}
	if got := MutationTargets([]string{path}); len(got) != 0 {
		t.Fatalf("generic app bootstrap produced mutation targets=%v", got)
	}
}
