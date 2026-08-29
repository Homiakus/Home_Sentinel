package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (a *app) build() error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("Go is required")
	}
	if err := a.ensureDirs(); err != nil {
		return err
	}
	if err := a.requireGoVersion(); err != nil {
		return err
	}
	module, err := output(a.root, nil, "go", "list", "-m", "-f", "{{.Path}}")
	if err != nil {
		return err
	}
	commit := "unknown"
	if s, err := output(a.root, nil, "git", "rev-parse", "--short=12", "HEAD"); err == nil {
		commit = s
	}
	version := envOr("SENTINEL_VERSION", "dev")
	built := time.Now().UTC().Format(time.RFC3339)
	ld := fmt.Sprintf("-s -w -X %s/internal/buildinfo.Version=%s -X %s/internal/buildinfo.Commit=%s -X %s/internal/buildinfo.BuildTime=%s", module, version, module, commit, module, built)
	fmt.Printf("Building Home Sentinel (%s, %s)...\n", version, commit)
	env := setEnv(os.Environ(), "CGO_ENABLED", "0")
	if err := stream(a.root, env, "go", "build", "-trimpath", "-ldflags="+ld, "-o", a.binary, "./cmd/sentinel"); err != nil {
		return err
	}
	_ = os.Chmod(a.binary, 0755)
	return stream(a.root, nil, a.binary, "version")
}

func (a *app) image() error {
	if err := dockerReady(); err != nil {
		return err
	}
	return stream(a.root, nil, "docker", "build", "-f", filepath.Join(a.root, "deploy", "compose", "Dockerfile"), "-t", a.localImage, a.root)
}

func (a *app) check() error {
	var files []string
	err := filepath.Walk(a.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && (info.Name() == ".git" || info.Name() == "vendor") {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)
	var unformatted []string
	for _, file := range files {
		got, err := output(a.root, nil, "gofmt", "-l", file)
		if err != nil {
			return err
		}
		if got != "" {
			unformatted = append(unformatted, got)
		}
	}
	if len(unformatted) > 0 {
		return fmt.Errorf("gofmt required:\n%s", strings.Join(unformatted, "\n"))
	}
	for _, step := range [][]string{
		{"go", "vet", "./..."},
		{"go", "test", "./..."},
		{"go", "run", "./cmd/sentinel-supplychain", "--root", "."},
		{"go", "run", "./cmd/sentinel-engloop", "reconcile", "--root", "."},
	} {
		if err := stream(a.root, nil, step[0], step[1:]...); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) doctor() error {
	failures, warnings := 0, 0
	fmt.Println("Home Sentinel doctor")
	if _, err := exec.LookPath("go"); err != nil {
		fmt.Println(" [FAIL] Go is not installed")
		failures++
	} else if err := a.requireGoVersion(); err != nil {
		fmt.Println(" [FAIL]", err)
		failures++
	} else if v, err := output(a.root, nil, "go", "version"); err == nil {
		fmt.Println(" [ OK ]", v)
	}
	if _, err := exec.LookPath("git"); err == nil {
		fmt.Println(" [ OK ] git")
	} else {
		fmt.Println(" [WARN] git not found")
		warnings++
	}
	if b, err := os.ReadFile(a.configFile); err != nil {
		fmt.Println(" [WARN] config missing; run setup")
		warnings++
	} else if !json.Valid(b) {
		fmt.Println(" [FAIL] config is not valid JSON")
		failures++
	} else {
		fmt.Println(" [ OK ] config:", a.configFile)
	}
	if _, err := os.Stat(a.binary); err == nil {
		fmt.Println(" [ OK ] binary built")
	} else {
		fmt.Println(" [WARN] binary not built")
		warnings++
	}
	a.cleanupStalePID()
	if pid, ok := a.runningPID(); ok {
		fmt.Printf(" [ OK ] process running (PID %d)\n", pid)
	} else {
		fmt.Println(" [INFO] process stopped")
	}
	if !portAvailable("127.0.0.1:8080") {
		if _, ok := a.runningPID(); !ok {
			fmt.Println(" [WARN] TCP/8080 occupied by another process")
			warnings++
		}
	} else {
		fmt.Println(" [ OK ] TCP/8080 available")
	}
	if err := dockerReady(); err != nil {
		fmt.Println(" [INFO] Docker unavailable:", err)
	} else {
		fmt.Println(" [ OK ] Docker daemon available")
	}
	if err := a.stackConfig(); err != nil {
		fmt.Println(" [WARN] Compose stack:", err)
		warnings++
	} else {
		fmt.Println(" [ OK ] Compose stack inputs/security valid")
	}
	fmt.Printf("Result: %d failure(s), %d warning(s).\n", failures, warnings)
	if failures > 0 {
		return fmt.Errorf("doctor found %d blocking failure(s)", failures)
	}
	return nil
}

func (a *app) update() error {
	if _, err := exec.LookPath("git"); err != nil {
		return err
	}
	dirty, err := output(a.root, nil, "git", "status", "--porcelain")
	if err != nil {
		return err
	}
	if dirty != "" {
		return errors.New("working tree has local changes; refusing automatic update")
	}
	_, wasRunning := a.runningPID()
	if err := stream(a.root, nil, "git", "pull", "--ff-only"); err != nil {
		return err
	}
	if err := a.build(); err != nil {
		return err
	}
	if wasRunning {
		return a.restartHost()
	}
	return nil
}
