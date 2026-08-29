package main

import "testing"

func TestParseProxyConfig(t *testing.T) {
	if _, err := parseProxyConfig([]string{"--listen", "0.0.0.0:8080", "--upstream", "http://127.0.0.1:18080"}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"http://10.0.0.2:18080", "ftp://127.0.0.1:18080", "http://example.com"} {
		if _, err := parseProxyConfig([]string{"--upstream", bad}); err == nil {
			t.Fatalf("expected %q rejected", bad)
		}
	}
}
