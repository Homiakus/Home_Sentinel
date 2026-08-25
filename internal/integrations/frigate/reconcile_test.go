package frigate

import (
	fgconfig "github.com/Homiakus/Home_Sentinel/internal/integrations/frigate/config"
	"testing"
)

func TestReconcileIgnoresManualButFindsManagedDrift(t *testing.T) {
	desired := []byte(`{"cameras":{"cam_a":{"x":1}},"go2rtc":{"streams":{"cam_a":["x"]}}}`)
	actual := map[string]any{"cameras": map[string]any{"cam_a": map[string]any{"x": float64(2)}, "manual": map[string]any{"x": 9}}, "go2rtc": map[string]any{"streams": map[string]any{"cam_a": []any{"x"}, "manual": []any{"y"}}}}
	r := Reconcile(desired, actual, fgconfig.Ownership{CameraNames: []string{"cam_a"}, StreamNames: []string{"cam_a"}})
	if r.InSync || len(r.Drift) != 1 || r.Drift[0].Resource != "camera:cam_a" {
		t.Fatalf("%+v", r)
	}
}
