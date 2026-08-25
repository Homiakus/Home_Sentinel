package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	upd "github.com/Homiakus/Home_Sentinel/internal/update"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type commandRunner struct{}

func (commandRunner) run(ctx context.Context, name string, args ...string) error {
	c := exec.CommandContext(ctx, name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

type composeExec struct {
	compose, currentEnv, targetEnv, checkpointPath, healthURL string
	r                                                         commandRunner
}

func (c *composeExec) base(env string) []string {
	return []string{"compose", "--env-file", env, "-f", c.compose}
}
func (c *composeExec) Stage(ctx context.Context, p upd.Plan) error {
	a := append(c.base(c.targetEnv), "pull")
	return c.r.run(ctx, "docker", a...)
}
func (c *composeExec) Activate(ctx context.Context, p upd.Plan) error {
	a := append(c.base(c.targetEnv), "up", "-d", "--remove-orphans")
	return c.r.run(ctx, "docker", a...)
}
func (c *composeExec) Verify(ctx context.Context, p upd.Plan) error {
	client := http.Client{Timeout: 3 * time.Second}
	var last error
	for i := 0; i < 20; i++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", c.healthURL, nil)
		resp, e := client.Do(req)
		if e == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
			last = fmt.Errorf("readiness HTTP %d", resp.StatusCode)
		} else {
			last = e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("readiness did not recover: %w", last)
}
func (c *composeExec) Rollback(ctx context.Context, p upd.Plan) error {
	a := append(c.base(c.targetEnv), "stop", "sentinel")
	return c.r.run(ctx, "docker", a...)
}

type composeCheckpoint struct{ exec *composeExec }

func (c composeCheckpoint) Create(ctx context.Context) (string, error) {
	if c.exec.checkpointPath == "" {
		return "", errors.New("checkpoint path required")
	}
	a := append(c.exec.base(c.exec.currentEnv), "exec", "-T", "sentinel", "sentinel", "checkpoint", "--out", c.exec.checkpointPath)
	if e := c.exec.r.run(ctx, "docker", a...); e != nil {
		return "", e
	}
	return c.exec.checkpointPath, nil
}
func (c composeCheckpoint) Restore(ctx context.Context, id string) error {
	a := append(c.exec.base(c.exec.currentEnv), "run", "--rm", "--no-deps", "sentinel", "sentinel", "restore-checkpoint", "--from", id)
	if e := c.exec.r.run(ctx, "docker", a...); e != nil {
		return e
	}
	a = append(c.exec.base(c.exec.currentEnv), "up", "-d", "--remove-orphans")
	return c.exec.r.run(ctx, "docker", a...)
}
func readJSON(path string, dst any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
func main() {
	if len(os.Args) < 2 {
		fatal("usage: sentinel-updater <inventory|plan|apply>")
	}
	command := os.Args[1]
	if command == "inventory" {
		fs := flag.NewFlagSet(command, flag.ExitOnError)
		envPath := fs.String("env", "", "current release env file")
		systemURL := fs.String("system-url", "http://127.0.0.1:8080", "Sentinel base URL")
		out := fs.String("out", "", "current-state JSON output; stdout when empty")
		_ = fs.Parse(os.Args[2:])
		if *envPath == "" {
			fatal("inventory requires --env")
		}
		f, e := os.Open(*envPath)
		if e != nil {
			fatal(e.Error())
		}
		env, e := upd.ParseEnv(f)
		_ = f.Close()
		if e != nil {
			fatal(e.Error())
		}
		info, e := upd.FetchSystemReleaseInfo(&http.Client{Timeout: 5 * time.Second}, *systemURL)
		if e != nil {
			fatal(e.Error())
		}
		cur, e := upd.Inventory(env, info)
		if e != nil {
			fatal(e.Error())
		}
		var w io.Writer = os.Stdout
		if *out != "" {
			x, xerr := os.Create(*out)
			if xerr != nil {
				fatal(xerr.Error())
			}
			defer x.Close()
			w = x
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if e = enc.Encode(cur); e != nil {
			fatal(e.Error())
		}
		return
	}

	fs := flag.NewFlagSet(command, flag.ExitOnError)
	var currentPath, manifestPath, compose, currentEnv, targetEnv, cp, health string
	fs.StringVar(&currentPath, "current", "", "current-state JSON")
	fs.StringVar(&manifestPath, "manifest", "", "target release manifest")
	fs.StringVar(&compose, "compose", "deploy/compose/compose.prod.yml", "compose file")
	fs.StringVar(&currentEnv, "current-env", "", "current release env file")
	fs.StringVar(&targetEnv, "target-env", "", "target release env file")
	fs.StringVar(&cp, "checkpoint", "/var/lib/home-sentinel/update-checkpoints/pre-update", "checkpoint path inside sentinel volume")
	fs.StringVar(&health, "health-url", "http://127.0.0.1:8080/readyz", "readiness URL")
	_ = fs.Parse(os.Args[2:])
	if currentPath == "" || manifestPath == "" {
		fatal("--current and --manifest are required")
	}
	var cur upd.Current
	var man upd.Manifest
	if e := readJSON(currentPath, &cur); e != nil {
		fatal(e.Error())
	}
	if e := readJSON(manifestPath, &man); e != nil {
		fatal(e.Error())
	}
	p, e := upd.BuildPlan(cur, man)
	if e != nil {
		fatal(e.Error())
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if command == "plan" {
		_ = enc.Encode(p)
		if !p.Compatible {
			os.Exit(2)
		}
		return
	}
	if command != "apply" {
		fatal("unknown command")
	}
	if !p.Compatible {
		fatal("incompatible update: " + strings.Join(p.Reasons, "; "))
	}
	if currentEnv == "" || targetEnv == "" {
		fatal("apply requires --current-env and --target-env")
	}
	x := &composeExec{compose: compose, currentEnv: currentEnv, targetEnv: targetEnv, checkpointPath: cp, healthURL: health}
	res, e := (upd.Manager{Checkpoint: composeCheckpoint{x}, Executor: x}).Apply(context.Background(), p)
	_ = enc.Encode(res)
	if e != nil {
		fatal(e.Error())
	}
}
func fatal(s string) { fmt.Fprintln(os.Stderr, "sentinel-updater:", s); os.Exit(1) }
