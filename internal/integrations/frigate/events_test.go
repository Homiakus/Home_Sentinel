package frigate

import "testing"

func TestMediaReferenceIsReferenceOnly(t *testing.T) {
	m := ReferenceMedia("123.abc-x")
	if m.EventID == "" || m.SnapshotPath != "/api/events/123.abc-x/snapshot.jpg" {
		t.Fatalf("%+v", m)
	}
	if ReferenceMedia("../etc/passwd").EventID != "" {
		t.Fatal("unsafe id accepted")
	}
}
