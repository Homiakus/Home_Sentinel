package config

import "testing"

func TestDefaultValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidCIDR(t *testing.T) {
	c := Default()
	c.Network.CameraCIDRs = []string{"nope"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected invalid CIDR")
	}
}
