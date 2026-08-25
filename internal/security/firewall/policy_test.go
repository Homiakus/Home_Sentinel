package firewall

import (
	"strings"
	"testing"
)

func TestRenderNFTRestrictsOwnedPorts(t *testing.T) {
	p := DefaultPolicy()
	p.TrustedCIDRs = []string{"192.168.1.0/24", "fd00:1::/64"}
	p.CameraCIDRs = []string{"192.168.30.0/24"}
	p.CameraEgressCIDRs = []string{"172.30.252.0/24"}
	p.EnforceCameraEgress = true
	got, err := RenderNFT(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"table inet home_sentinel", "tcp dport { 8080, 8555 }", "udp dport 8555", "ip saddr @camera_egress_v4 ip daddr != @camera_v4 reject", "policy accept"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "policy drop") {
		t.Fatal("must not replace global host firewall policy")
	}
}
func TestPolicyRejectsMissingTrusted(t *testing.T) {
	p := DefaultPolicy()
	p.CameraCIDRs = []string{"10.0.0.0/24"}
	if p.Validate() == nil {
		t.Fatal("expected error")
	}
}
func TestPolicyRequiresCameraEgressCIDRWhenEnforced(t *testing.T) {
	p := DefaultPolicy()
	p.TrustedCIDRs = []string{"10.0.0.0/24"}
	p.CameraCIDRs = []string{"10.2.0.0/24"}
	p.EnforceCameraEgress = true
	if p.Validate() == nil {
		t.Fatal("expected error")
	}
}
