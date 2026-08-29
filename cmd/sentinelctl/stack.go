package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func (a *app) stackConfig() error {
	if err := dockerReady(); err != nil {
		return err
	}
	env, err := readDotEnv(a.envFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", a.envFile, err)
	}
	for _, key := range []string{"SENTINEL_IMAGE", "MOSQUITTO_IMAGE", "HOMEASSISTANT_IMAGE", "FRIGATE_IMAGE", "OLLAMA_IMAGE"} {
		v := strings.TrimSpace(env[key])
		if v == "" || strings.Contains(v, "REPLACE_ME") || strings.Contains(v, "registry.example.invalid") {
			return fmt.Errorf("%s must be a reviewed exact image tag/digest", key)
		}
	}
	compose, err := os.ReadFile(a.composeFile)
	if err != nil {
		return err
	}
	if err := validateComposeSecurity(string(compose)); err != nil {
		return err
	}
	if strings.TrimSpace(env["FRIGATE_WEBRTC_BIND_IP"]) == "" || strings.TrimSpace(env["FRIGATE_WEBRTC_CANDIDATES"]) == "" {
		return errors.New("FRIGATE_WEBRTC_BIND_IP and FRIGATE_WEBRTC_CANDIDATES are required")
	}
	for _, key := range []string{"SENTINEL_MQTT_PASSWORD_FILE", "FRIGATE_MQTT_PASSWORD_FILE", "HOMEASSISTANT_MQTT_PASSWORD_FILE"} {
		p := strings.TrimSpace(env[key])
		if p == "" {
			return fmt.Errorf("%s is required", key)
		}
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("%s points to unavailable file %q", key, p)
		}
	}
	return a.compose("config", "--quiet")
}

func validateComposeSecurity(compose string) error {
	for _, required := range []string{
		`"127.0.0.1:8080:8080"`,
		`SENTINEL_LISTEN: 127.0.0.1:18080`,
		`network_mode: "service:sentinel"`,
		`command: ["proxy", "--listen", "0.0.0.0:8080", "--upstream", "http://127.0.0.1:18080"]`,
	} {
		if !strings.Contains(compose, required) {
			return fmt.Errorf("production Compose security contract missing %q", required)
		}
	}
	if strings.Contains(compose, `${SENTINEL_BIND_IP`) {
		return errors.New("Sentinel control-plane host bind must not be configurable")
	}
	return nil
}

func (a *app) stackUp() error {
	if err := a.stackConfig(); err != nil {
		return err
	}
	return a.compose("up", "-d")
}

func (a *app) compose(args ...string) error {
	if err := dockerReady(); err != nil {
		return err
	}
	base := []string{"compose", "--env-file", a.envFile, "-f", a.composeFile}
	return stream(a.root, nil, "docker", append(base, args...)...)
}

func dockerReady() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("Docker is not installed")
	}
	for _, args := range [][]string{{"info"}, {"compose", "version"}} {
		cmd := exec.Command("docker", args...)
		cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
		if err := cmd.Run(); err != nil {
			if len(args) == 1 {
				return errors.New("Docker daemon is unavailable")
			}
			return errors.New("Docker Compose v2 plugin is unavailable")
		}
	}
	return nil
}

func readDotEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		out[k] = v
	}
	return out, s.Err()
}
