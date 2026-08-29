package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const serviceName = "home-sentinel"

var defaultConfig = []byte(`{
  "server": {
    "listen": "127.0.0.1:8080",
    "read_timeout": "15s",
    "write_timeout": "30s",
    "shutdown_grace": "10s"
  },
  "database": {"busy_timeout": "5s"},
  "features": {"experimental": false}
}
`)

type app struct {
	root, binDir, binary, varDir, configFile, dataDir string
	logDir, runDir, logFile, pidFile, envFile         string
	composeFile, localImage                           string
}

func newApp() (*app, error) {
	root, err := findRepoRoot()
	if err != nil {
		return nil, err
	}
	binDir := envOr("SENTINELCTL_BIN_DIR", filepath.Join(root, "bin"))
	varDir := envOr("SENTINELCTL_VAR_DIR", filepath.Join(root, "var"))
	return &app{
		root:        root,
		binDir:      binDir,
		binary:      envOr("SENTINELCTL_BINARY", filepath.Join(binDir, executableName("sentinel"))),
		varDir:      varDir,
		configFile:  envOr("SENTINELCTL_CONFIG", filepath.Join(varDir, "config.json")),
		dataDir:     filepath.Join(varDir, "data"),
		logDir:      filepath.Join(varDir, "log"),
		runDir:      filepath.Join(varDir, "run"),
		logFile:     filepath.Join(varDir, "log", "sentinel.log"),
		pidFile:     filepath.Join(varDir, "run", "sentinel.pid"),
		envFile:     envOr("SENTINELCTL_ENV_FILE", filepath.Join(root, ".env")),
		composeFile: filepath.Join(root, "deploy", "compose", "compose.prod.yml"),
		localImage:  envOr("SENTINELCTL_IMAGE", "home-sentinel:local"),
	}, nil
}

func findRepoRoot() (string, error) {
	var starts []string
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		for dir := start; ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				if _, err := os.Stat(filepath.Join(dir, "cmd", "sentinel")); err == nil {
					return dir, nil
				}
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return "", errors.New("repository root not found; run from the Home_Sentinel checkout")
}

func (a *app) ensureDirs() error {
	for _, dir := range []string{a.binDir, a.varDir, a.dataDir, a.logDir, a.runDir, filepath.Join(a.varDir, "frigate-secrets")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		_ = os.Chmod(dir, 0700)
	}
	return nil
}

func (a *app) setup() error {
	if err := a.ensureDirs(); err != nil {
		return err
	}
	if _, err := os.Stat(a.configFile); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(a.configFile, defaultConfig, 0600); err != nil {
			return err
		}
		fmt.Println("Created", a.configFile)
	}
	if _, err := os.Stat(a.envFile); errors.Is(err, os.ErrNotExist) {
		if b, readErr := os.ReadFile(filepath.Join(a.root, ".env.example")); readErr == nil {
			if err := os.WriteFile(a.envFile, b, 0600); err != nil {
				return err
			}
			fmt.Println("Created", a.envFile, "from .env.example; review production pins before stack-up")
		}
	}
	fmt.Printf("Local runtime initialized.\n config: %s\n data:   %s\n logs:   %s\n", a.configFile, a.dataDir, a.logFile)
	return nil
}

func (a *app) configure() error {
	if err := a.setup(); err != nil {
		return err
	}
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		switch runtime.GOOS {
		case "windows":
			editor = "notepad"
		case "darwin":
			editor = "open -e"
		default:
			editor = "vi"
		}
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return errors.New("EDITOR is empty")
	}
	cmd := exec.Command(parts[0], append(parts[1:], a.configFile)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func (a *app) runtimeEnv() []string {
	env := append([]string{}, os.Environ()...)
	env = setEnv(env, "SENTINEL_CONFIG", a.configFile)
	env = setEnv(env, "SENTINEL_DB_PATH", filepath.Join(a.dataDir, "sentinel.db"))
	if _, ok := os.LookupEnv("SENTINEL_LISTEN"); !ok {
		env = setEnv(env, "SENTINEL_LISTEN", "127.0.0.1:8080")
	}
	if _, ok := os.LookupEnv("SENTINEL_FRIGATE_CREDENTIALS_DIR"); !ok {
		env = setEnv(env, "SENTINEL_FRIGATE_CREDENTIALS_DIR", filepath.Join(a.varDir, "frigate-secrets"))
	}
	return env
}
