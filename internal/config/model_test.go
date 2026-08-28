package config

import (
	"errors"
	"testing"
)

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

func TestRuntimeServerListenAddressFailsClosedOutsideLoopback(t *testing.T) {
	for _, address := range []string{
		"127.0.0.1:8080",
		"127.42.0.9:8080",
		"localhost:8080",
		"[::1]:8080",
	} {
		t.Run("allow_"+address, func(t *testing.T) {
			cfg := Default()
			cfg.Server.ListenAddress = address
			if err := cfg.Validate(); err != nil {
				t.Fatalf("loopback address %q rejected: %v", address, err)
			}
		})
	}

	for _, address := range []string{
		"0.0.0.0:8080",
		"[::]:8080",
		"192.168.1.10:8080",
		"10.0.0.10:8080",
		"example.com:8080",
		":8080",
	} {
		t.Run("deny_"+address, func(t *testing.T) {
			cfg := Default()
			cfg.Server.ListenAddress = address
			if err := cfg.Validate(); !errors.Is(err, ErrInsecureRemoteBind) {
				t.Fatalf("remote/plaintext address %q was not rejected with ErrInsecureRemoteBind: %v", address, err)
			}
		})
	}
}

func TestRuntimeServerListenAddressRejectsMalformedHostPort(t *testing.T) {
	cfg := Default()
	cfg.Server.ListenAddress = "127.0.0.1"
	if err := cfg.Validate(); err == nil {
		t.Fatal("malformed listen address accepted")
	} else if errors.Is(err, ErrInsecureRemoteBind) {
		t.Fatalf("malformed listen address misclassified as remote bind: %v", err)
	}
}
