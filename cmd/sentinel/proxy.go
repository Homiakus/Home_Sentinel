package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type proxyConfig struct {
	Listen   string
	Upstream *url.URL
}

func parseProxyConfig(args []string) (proxyConfig, error) {
	fs := flag.NewFlagSet("proxy", flag.ContinueOnError)
	listen := fs.String("listen", "0.0.0.0:8080", "listen address")
	upstream := fs.String("upstream", "http://127.0.0.1:18080", "loopback upstream")
	if err := fs.Parse(args); err != nil {
		return proxyConfig{}, err
	}
	if fs.NArg() != 0 {
		return proxyConfig{}, fmt.Errorf("unexpected proxy arguments: %s", strings.Join(fs.Args(), " "))
	}
	if _, _, err := net.SplitHostPort(strings.TrimSpace(*listen)); err != nil {
		return proxyConfig{}, errors.New("proxy listen must be host:port")
	}
	u, err := url.Parse(strings.TrimSpace(*upstream))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return proxyConfig{}, errors.New("proxy upstream must be an http(s) URL")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return proxyConfig{}, errors.New("proxy upstream must resolve to an explicit loopback host")
	}
	return proxyConfig{Listen: strings.TrimSpace(*listen), Upstream: u}, nil
}

func runProxy(args []string) error {
	cfg, err := parseProxyConfig(args)
	if err != nil {
		return err
	}
	rp := httputil.NewSingleHostReverseProxy(cfg.Upstream)
	original := rp.Director
	rp.Director = func(r *http.Request) {
		r.Header.Del("Forwarded")
		r.Header.Del("X-Forwarded-For")
		r.Header.Del("X-Forwarded-Host")
		r.Header.Del("X-Forwarded-Proto")
		original(r)
		r.Header.Set("X-Forwarded-Proto", "http")
	}
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}
	srv := &http.Server{Addr: cfg.Listen, Handler: rp, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	errCh := make(chan error, 1)
	go func() {
		log.Info("sentinel ingress starting", "listen", cfg.Listen, "upstream", cfg.Upstream.String())
		errCh <- srv.ListenAndServe()
	}()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	select {
	case sig := <-sigCh:
		log.Info("sentinel ingress shutdown requested", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
