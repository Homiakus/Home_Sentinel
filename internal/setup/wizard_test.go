package setup

import "testing"

func TestWizardRequiresCameraAndCore(t *testing.T) {
	s := WizardSnapshot{Admin: true, Storage: true, Network: true, FrigateEnabled: true, FrigateHealthy: true}
	w := EvaluateWizard(s)
	if w.Complete {
		t.Fatal("camera must be required")
	}
	s.CameraCount = 1
	w = EvaluateWizard(s)
	if !w.Complete {
		t.Fatalf("expected complete %+v", w)
	}
}
func TestDisabledOptionalStepsSkipped(t *testing.T) {
	w := EvaluateWizard(WizardSnapshot{Admin: true, Storage: true, Network: true, CameraCount: 1})
	for _, s := range w.Steps {
		if s.ID == "telegram" && s.Status != Skipped {
			t.Fatalf("telegram=%s", s.Status)
		}
	}
}
