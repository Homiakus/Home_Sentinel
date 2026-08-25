package rtsp

import (
	"context"
	"errors"
	"net"
	"net/url"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/media"
)

type Result struct {
	Reachable bool              `json:"reachable"`
	TCPTime   time.Duration     `json:"tcp_time"`
	Media     media.ProbeResult `json:"media"`
}

func Probe(ctx context.Context, rawURL string, timeout time.Duration) (Result, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "rtsp" {
		return Result{}, errors.New("valid rtsp URL required")
	}
	host := u.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(u.Hostname(), "554")
	}
	d := net.Dialer{Timeout: minDuration(timeout, 3*time.Second)}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return Result{}, err
	}
	_ = conn.Close()
	r := Result{Reachable: true, TCPTime: time.Since(start)}
	r.Media, err = media.Probe(ctx, rawURL, timeout)
	return r, err
}
func minDuration(a, b time.Duration) time.Duration {
	if a <= 0 {
		return b
	}
	if a < b {
		return a
	}
	return b
}
