package frigate

import (
	fgconfig "github.com/Homiakus/Home_Sentinel/internal/integrations/frigate/config"
	"testing"
)

func TestPolicyMapping(t *testing.T) {
	c := fgconfig.Camera{}
	p := DefaultCameraPolicy()
	p.Zones = []ZonePolicy{{Name: "door", Coordinates: "0,0,1,0,1,1", Objects: []string{"person"}}}
	if err := ApplyPolicy(&c, p); err != nil {
		t.Fatal(err)
	}
	if c.Record == nil || c.Record.Alerts.Retain.Days != 30 || c.Snapshots.Retain.Default != 30 || c.Zones["door"].Coordinates == "" {
		t.Fatalf("bad policy %+v", c)
	}
}
