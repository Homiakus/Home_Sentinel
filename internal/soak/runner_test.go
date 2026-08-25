package soak

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestShortSoakPass(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer s.Close()
	r, e := (Runner{}).Run(context.Background(), Options{BaseURL: s.URL, Duration: 45 * time.Millisecond, Interval: 10 * time.Millisecond, RequestTimeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	if !r.Passed || r.Samples < 2 {
		t.Fatalf("%+v", r)
	}
}
func TestReadinessFailuresBlock(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	}))
	defer s.Close()
	r, e := (Runner{}).Run(context.Background(), Options{BaseURL: s.URL, Duration: 25 * time.Millisecond, Interval: 10 * time.Millisecond, RequestTimeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	if r.Passed || r.ReadyFailures == 0 {
		t.Fatalf("%+v", r)
	}
}
