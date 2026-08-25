package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const maxRESTBody = 8 << 20

var entityIDPattern = regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)

type Action struct {
	Domain  string
	Service string
}

func (a Action) String() string { return a.Domain + "." + a.Service }

type RESTOptions struct {
	BaseURL        string
	Token          string
	HTTPClient     *http.Client
	AllowedActions []Action
}

type RESTClient struct {
	base    *url.URL
	token   string
	http    *http.Client
	allowed map[string]struct{}
}

type APIInfo struct {
	Message string `json:"message"`
}

type ConfigInfo struct {
	Components   []string `json:"components"`
	LocationName string   `json:"location_name"`
	TimeZone     string   `json:"time_zone"`
	Version      string   `json:"version"`
}

type State struct {
	EntityID    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes"`
	LastChanged string         `json:"last_changed"`
	LastUpdated string         `json:"last_updated"`
}

type HTTPError struct {
	Status int
	Method string
	Path   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Home Assistant %s %s returned HTTP %d", e.Method, e.Path, e.Status)
}

func NewRESTClient(opts RESTOptions) (*RESTClient, error) {
	if strings.TrimSpace(opts.Token) == "" {
		return nil, errors.New("Home Assistant token required")
	}
	u, err := url.Parse(strings.TrimSpace(opts.BaseURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("Home Assistant base URL invalid")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("Home Assistant base URL must use http or https")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	allowed := make(map[string]struct{}, len(opts.AllowedActions))
	for _, action := range opts.AllowedActions {
		if err := validateAction(action); err != nil {
			return nil, err
		}
		allowed[action.String()] = struct{}{}
	}
	return &RESTClient{base: u, token: opts.Token, http: client, allowed: allowed}, nil
}

func validateAction(a Action) error {
	valid := regexp.MustCompile(`^[a-z0-9_]+$`)
	if !valid.MatchString(a.Domain) || !valid.MatchString(a.Service) {
		return fmt.Errorf("invalid Home Assistant action %q", a.String())
	}
	return nil
}

func (c *RESTClient) Ping(ctx context.Context) (APIInfo, error) {
	var out APIInfo
	return out, c.doJSON(ctx, http.MethodGet, "/api/", nil, &out)
}

func (c *RESTClient) Config(ctx context.Context) (ConfigInfo, error) {
	var out ConfigInfo
	return out, c.doJSON(ctx, http.MethodGet, "/api/config", nil, &out)
}

func (c *RESTClient) States(ctx context.Context) ([]State, error) {
	var out []State
	return out, c.doJSON(ctx, http.MethodGet, "/api/states", nil, &out)
}

func (c *RESTClient) State(ctx context.Context, entityID string) (State, error) {
	var out State
	if !entityIDPattern.MatchString(entityID) {
		return out, errors.New("invalid Home Assistant entity id")
	}
	return out, c.doJSON(ctx, http.MethodGet, "/api/states/"+entityID, nil, &out)
}

func (c *RESTClient) CallAction(ctx context.Context, action Action, payload map[string]any) ([]State, error) {
	if err := validateAction(action); err != nil {
		return nil, err
	}
	if _, ok := c.allowed[action.String()]; !ok {
		return nil, fmt.Errorf("Home Assistant action %s is not allowlisted", action.String())
	}
	if payload == nil {
		payload = map[string]any{}
	}
	var out []State
	err := c.doJSON(ctx, http.MethodPost, "/api/services/"+action.Domain+"/"+action.Service, payload, &out)
	return out, err
}

func (c *RESTClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	if c == nil || c.base == nil {
		return errors.New("Home Assistant REST client unavailable")
	}
	if !strings.HasPrefix(path, "/api/") || strings.Contains(path, ".storage") || strings.Contains(path, "..") {
		return errors.New("Home Assistant path rejected")
	}
	u := *c.base
	u.Path = strings.TrimRight(c.base.Path, "/") + path
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Home Assistant request: %w", err)
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxRESTBody+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(data) > maxRESTBody {
		return errors.New("Home Assistant response too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{Status: resp.StatusCode, Method: method, Path: path}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode Home Assistant response: %w", err)
	}
	return nil
}

func HasComponent(info ConfigInfo, component string) bool {
	for _, c := range info.Components {
		if c == component {
			return true
		}
	}
	return false
}
