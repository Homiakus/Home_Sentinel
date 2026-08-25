package update

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var componentEnv = map[string]string{
	"sentinel":      "SENTINEL_IMAGE",
	"mosquitto":     "MOSQUITTO_IMAGE",
	"homeassistant": "HOMEASSISTANT_IMAGE",
	"frigate":       "FRIGATE_IMAGE",
	"ollama":        "OLLAMA_IMAGE",
}

func ParseEnv(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(io.LimitReader(r, 1<<20))
	for lineNo := 1; sc.Scan(); lineNo++ {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "export "))
		key, value, ok := strings.Cut(raw, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("env line %d is not KEY=VALUE", lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
			value = value[1 : len(value)-1]
		}
		out[key] = value
	}
	return out, sc.Err()
}

func ComponentsFromEnv(env map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for component, key := range componentEnv {
		ref := strings.TrimSpace(env[key])
		if ref == "" {
			return nil, fmt.Errorf("%s is missing", key)
		}
		if !ExactRef(ref) {
			return nil, fmt.Errorf("%s must be an exact non-latest tag or digest", key)
		}
		out[component] = ref
	}
	return out, nil
}

type SystemReleaseInfo struct {
	Version       string `json:"version"`
	SchemaVersion int64  `json:"schema_version"`
}

func FetchSystemReleaseInfo(client *http.Client, url string) (SystemReleaseInfo, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(url, "/")+"/api/v1/system", nil)
	if err != nil {
		return SystemReleaseInfo{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return SystemReleaseInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return SystemReleaseInfo{}, fmt.Errorf("system endpoint HTTP %d", resp.StatusCode)
	}
	var info SystemReleaseInfo
	dec := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
	if err = dec.Decode(&info); err != nil {
		return info, err
	}
	if strings.TrimSpace(info.Version) == "" || info.SchemaVersion <= 0 {
		return info, errors.New("system endpoint returned incomplete release info")
	}
	return info, nil
}

func Inventory(env map[string]string, info SystemReleaseInfo) (Current, error) {
	components, err := ComponentsFromEnv(env)
	if err != nil {
		return Current{}, err
	}
	if _, err := semver(info.Version); err != nil {
		return Current{}, fmt.Errorf("sentinel version: %w", err)
	}
	if info.SchemaVersion <= 0 {
		return Current{}, errors.New("schema version must be positive")
	}
	return Current{Version: info.Version, SchemaVersion: info.SchemaVersion, Components: components}, nil
}
