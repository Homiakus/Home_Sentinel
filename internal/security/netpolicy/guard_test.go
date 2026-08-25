package netpolicy

import (
	"context"
	"testing"
)

func TestGuardLiteralIP(t *testing.T) {
	g, err := New([]string{"192.168.30.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := g.PinURL(context.Background(), "rtsp://192.168.30.20:554/live", "rtsp")
	if err != nil {
		t.Fatal(err)
	}
	if u != "rtsp://192.168.30.20:554/live" {
		t.Fatalf("url=%s", u)
	}
	if _, err := g.PinURL(context.Background(), "rtsp://127.0.0.1/live", "rtsp"); err == nil {
		t.Fatal("loopback accepted")
	}
	if _, err := g.PinURL(context.Background(), "http://192.168.30.20/live", "rtsp"); err == nil {
		t.Fatal("wrong scheme accepted")
	}
}
