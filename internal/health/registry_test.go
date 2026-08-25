package health

import "testing"

func TestDiagnosisSuppressesDependentFailure(t *testing.T) {
	r := NewRegistry()
	r.Set("mqtt", Failed, "BROKER_DOWN", "dial failed")
	r.Set("home_assistant", Degraded, "MQTT_UNAVAILABLE", "discovery unavailable")
	d := Diagnose(r, DependencyGraph{"home_assistant": {"mqtt"}})
	var found bool
	for _, x := range d {
		if x.Component.Name == "home_assistant" {
			found = true
			if x.SuppressedBy != "mqtt" || x.RootCause {
				t.Fatalf("bad diagnosis: %+v", x)
			}
		}
	}
	if !found {
		t.Fatal("home_assistant diagnosis missing")
	}
}
