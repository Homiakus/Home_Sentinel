package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareVersion(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"1.26.6", "1.26.6", 0}, {"1.27.0", "1.26.6", 1}, {"1.25.9", "1.26.0", -1}, {"1.26rc1", "1.26.0", 0},
	} {
		if got := compareVersion(tc.a, tc.b); got != tc.want {
			t.Fatalf("compareVersion(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestReadDotEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte("# x\nA=1\nB=\"two\"\nexport C='three'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := readDotEnv(p)
	if err != nil {
		t.Fatal(err)
	}
	if m["A"] != "1" || m["B"] != "two" || m["C"] != "three" {
		t.Fatalf("unexpected: %#v", m)
	}
}

func TestValidateComposeSecurity(t *testing.T) {
	good := `ports:
  - "127.0.0.1:8080:8080"
environment:
  SENTINEL_LISTEN: 127.0.0.1:18080
network_mode: "service:sentinel"
command: ["proxy", "--listen", "0.0.0.0:8080", "--upstream", "http://127.0.0.1:18080"]
`
	if err := validateComposeSecurity(good); err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(good, `"127.0.0.1:8080:8080"`, `"${SENTINEL_BIND_IP}:8080:8080"`, 1)
	if err := validateComposeSecurity(bad); err == nil {
		t.Fatal("expected configurable bind rejection")
	}
}
