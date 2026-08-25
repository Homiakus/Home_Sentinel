package frigate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 32 << 20

type NetworkError struct {
	Method string
	Path   string
	Err    error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("frigate %s %s network error: %v", e.Method, e.Path, e.Err)
}
func (e *NetworkError) Unwrap() error { return e.Err }

type HTTPError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("frigate %s %s returned HTTP %d", e.Method, e.Path, e.StatusCode)
}

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

type ClientOptions struct {
	BaseURL     string
	BearerToken string
	Timeout     time.Duration
	HTTPClient  *http.Client
}

func NewClient(opts ClientOptions) (*Client, error) {
	u, err := url.Parse(strings.TrimSpace(opts.BaseURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("invalid Frigate base URL")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: opts.Timeout, Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         (&net.Dialer{Timeout: 4 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        32,
			MaxIdleConnsPerHost: 8,
			IdleConnTimeout:     60 * time.Second,
		}}
	}
	return &Client{base: u, token: strings.TrimSpace(opts.BearerToken), http: hc}, nil
}

func (c *Client) endpoint(path string, query url.Values) string {
	u := *c.base
	u.Path = strings.TrimRight(c.base.Path, "/") + "/api/" + strings.TrimLeft(path, "/")
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte, contentType string) ([]byte, http.Header, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path, query), reader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, &NetworkError{Method: method, Path: path, Err: err}
	}
	defer resp.Body.Close()
	b, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if readErr != nil {
		return nil, resp.Header, &NetworkError{Method: method, Path: path, Err: readErr}
	}
	if len(b) > maxResponseBytes {
		return nil, resp.Header, &NetworkError{Method: method, Path: path, Err: errors.New("response exceeds safety limit")}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		excerpt := strings.TrimSpace(string(b))
		if len(excerpt) > 1024 {
			excerpt = excerpt[:1024]
		}
		return nil, resp.Header, &HTTPError{Method: method, Path: path, StatusCode: resp.StatusCode, Body: excerpt}
	}
	return b, resp.Header, nil
}

func (c *Client) Version(ctx context.Context) (string, error) {
	b, _, err := c.do(ctx, http.MethodGet, "version", nil, nil, "")
	if err != nil {
		return "", err
	}
	raw := strings.TrimSpace(string(b))
	var s string
	if json.Unmarshal(b, &s) == nil && s != "" {
		return s, nil
	}
	var obj struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(b, &obj) == nil && obj.Version != "" {
		return obj.Version, nil
	}
	return strings.Trim(raw, `"`), nil
}

func (c *Client) ConfigSchema(ctx context.Context) (json.RawMessage, error) {
	b, _, err := c.do(ctx, http.MethodGet, "config/schema.json", nil, nil, "")
	if err != nil {
		return nil, err
	}
	if !json.Valid(b) {
		return nil, errors.New("Frigate config schema response is not JSON")
	}
	return json.RawMessage(b), nil
}

func (c *Client) Config(ctx context.Context) (map[string]any, error) {
	b, _, err := c.do(ctx, http.MethodGet, "config", nil, nil, "")
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode Frigate config: %w", err)
	}
	return out, nil
}

func (c *Client) ConfigRaw(ctx context.Context) ([]byte, error) {
	b, _, err := c.do(ctx, http.MethodGet, "config/raw", nil, nil, "")
	return b, err
}

func (c *Client) SaveConfig(ctx context.Context, configJSON []byte, saveOnly bool) error {
	q := url.Values{}
	if saveOnly {
		q.Set("save_option", "saveonly")
	}
	_, _, err := c.do(ctx, http.MethodPost, "config/save", q, configJSON, "application/json")
	return err
}

func (c *Client) Restart(ctx context.Context) error {
	_, _, err := c.do(ctx, http.MethodPost, "restart", nil, nil, "application/json")
	return err
}

func (c *Client) Go2RTCStreams(ctx context.Context) (map[string]json.RawMessage, error) {
	b, _, err := c.do(ctx, http.MethodGet, "go2rtc/streams", nil, nil, "")
	if err != nil {
		return nil, err
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode go2rtc streams: %w", err)
	}
	return out, nil
}

func (c *Client) Events(ctx context.Context, query url.Values) ([]json.RawMessage, error) {
	b, _, err := c.do(ctx, http.MethodGet, "events", query, nil, "")
	if err != nil {
		return nil, err
	}
	var out []json.RawMessage
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode Frigate events: %w", err)
	}
	return out, nil
}

func (c *Client) Event(ctx context.Context, eventID string) (json.RawMessage, error) {
	if !safeID(eventID) {
		return nil, errors.New("invalid Frigate event id")
	}
	b, _, err := c.do(ctx, http.MethodGet, "events/"+eventID, nil, nil, "")
	return json.RawMessage(b), err
}

func (c *Client) EventSnapshot(ctx context.Context, eventID string) ([]byte, string, error) {
	if !safeID(eventID) {
		return nil, "", errors.New("invalid Frigate event id")
	}
	b, h, err := c.do(ctx, http.MethodGet, "events/"+eventID+"/snapshot.jpg", nil, nil, "")
	if err != nil {
		return nil, "", err
	}
	return b, h.Get("Content-Type"), nil
}

func (c *Client) EventClip(ctx context.Context, eventID string) ([]byte, string, error) {
	if !safeID(eventID) {
		return nil, "", errors.New("invalid Frigate event id")
	}
	b, h, err := c.do(ctx, http.MethodGet, "events/"+eventID+"/clip.mp4", nil, nil, "")
	if err != nil {
		return nil, "", err
	}
	return b, h.Get("Content-Type"), nil
}

func safeID(v string) bool {
	if v == "" || len(v) > 200 {
		return false
	}
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

// OpenMedia performs an authenticated, allowlisted GET against Frigate's media API.
// The caller owns resp.Body and must close it. This intentionally does not expose a
// generic reverse proxy: only explicit media paths accepted by safeMediaPath are allowed.
func (c *Client) OpenMedia(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	if c == nil {
		return nil, errors.New("Frigate client unavailable")
	}
	if !safeMediaPath(path) {
		return nil, errors.New("unsupported Frigate media path")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint(path, query), nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &NetworkError{Method: http.MethodGet, Path: path, Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, &HTTPError{Method: http.MethodGet, Path: path, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(b))}
	}
	return resp, nil
}

func safeMediaPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 3 && parts[0] == "events" && safeID(parts[1]) && (parts[2] == "snapshot.jpg" || parts[2] == "clip.mp4") {
		return true
	}
	if len(parts) == 2 && safeID(parts[0]) && (parts[1] == "latest.jpg" || parts[1] == "latest.webp") {
		return true
	}
	return false
}
