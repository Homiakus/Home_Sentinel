package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/security/redact"
)

type Runner interface {
	Run(context.Context, string, []string, []string) ([]byte, []byte, error)
}
type ExecRunner struct{ MaxOutput int }

func (r ExecRunner) Run(ctx context.Context, binary string, args, env []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(os.Environ(), env...)
	var out, er bytes.Buffer
	limit := r.MaxOutput
	if limit <= 0 {
		limit = 16 << 20
	}
	cmd.Stdout = &limitedWriter{W: &out, N: int64(limit)}
	cmd.Stderr = &limitedWriter{W: &er, N: int64(limit)}
	err := cmd.Run()
	return out.Bytes(), er.Bytes(), err
}

type limitedWriter struct {
	W *bytes.Buffer
	N int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	orig := len(p)
	if w.N <= 0 {
		return orig, nil
	}
	if int64(len(p)) > w.N {
		p = p[:w.N]
	}
	_, _ = w.W.Write(p)
	w.N -= int64(len(p))
	return orig, nil
}

type Client struct {
	Binary     string
	Repository string
	Password   []byte
	Runner     Runner
	TempDir    string
}
type Snapshot struct {
	Time     time.Time `json:"time"`
	ID       string    `json:"id"`
	ShortID  string    `json:"short_id"`
	Hostname string    `json:"hostname"`
	Paths    []string  `json:"paths"`
	Tags     []string  `json:"tags"`
}
type BackupResult struct {
	SnapshotID string          `json:"snapshot_id,omitempty"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}
type Retention struct{ KeepHourly, KeepDaily, KeepWeekly, KeepMonthly int }

func (c Client) validate() error {
	if strings.TrimSpace(c.Repository) == "" {
		return errors.New("restic repository required")
	}
	if len(c.Password) == 0 {
		return errors.New("restic password required")
	}
	return nil
}
func (c Client) run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	if err := c.validate(); err != nil {
		return nil, nil, err
	}
	binary := c.Binary
	if binary == "" {
		binary = "restic"
	}
	runner := c.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	root := c.TempDir
	if root == "" {
		root = os.TempDir()
	}
	dir, err := os.MkdirTemp(root, "sentinel-restic-")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(dir)
	pass := filepath.Join(dir, "password")
	if err := os.WriteFile(pass, c.Password, 0o600); err != nil {
		return nil, nil, err
	}
	env := []string{"RESTIC_REPOSITORY=" + c.Repository, "RESTIC_PASSWORD_FILE=" + pass, "RESTIC_PROGRESS_FPS=0"}
	out, stderr, err := runner.Run(ctx, binary, args, env)
	if err != nil {
		return out, stderr, fmt.Errorf("restic %s failed: %w: %s", firstArg(args), err, redact.String(truncate(stderr, 2048)))
	}
	return out, stderr, nil
}
func firstArg(v []string) string {
	if len(v) == 0 {
		return "command"
	}
	return v[0]
}
func truncate(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	return strings.TrimSpace(string(b))
}
func (c Client) Init(ctx context.Context) error { _, _, err := c.run(ctx, "init"); return err }
func (c Client) Snapshots(ctx context.Context) ([]Snapshot, error) {
	out, _, err := c.run(ctx, "snapshots", "--json")
	if err != nil {
		return nil, err
	}
	var s []Snapshot
	if err := json.Unmarshal(out, &s); err != nil {
		return nil, fmt.Errorf("decode restic snapshots: %w", err)
	}
	return s, nil
}
func (c Client) Backup(ctx context.Context, paths, tags, excludes []string) (BackupResult, error) {
	if len(paths) == 0 {
		return BackupResult{}, errors.New("backup paths required")
	}
	args := []string{"backup", "--json"}
	for _, t := range tags {
		if strings.TrimSpace(t) != "" {
			args = append(args, "--tag", t)
		}
	}
	for _, x := range excludes {
		if x != "" {
			args = append(args, "--exclude", x)
		}
	}
	args = append(args, "--")
	args = append(args, paths...)
	out, _, err := c.run(ctx, args...)
	if err != nil {
		return BackupResult{}, err
	}
	var snapshot string
	lines := bytes.Split(out, []byte{'\n'})
	for _, line := range lines {
		var m struct {
			MessageType string `json:"message_type"`
			SnapshotID  string `json:"snapshot_id"`
		}
		if json.Unmarshal(line, &m) == nil && m.SnapshotID != "" {
			snapshot = m.SnapshotID
		}
	}
	return BackupResult{SnapshotID: snapshot, Raw: append([]byte(nil), out...)}, nil
}
func (c Client) Check(ctx context.Context) error {
	_, _, err := c.run(ctx, "check", "--read-data-subset=1/20")
	return err
}
func (c Client) Restore(ctx context.Context, snapshot, target string) error {
	if snapshot == "" || target == "" {
		return errors.New("snapshot and restore target required")
	}
	_, _, err := c.run(ctx, "restore", snapshot, "--target", target)
	return err
}
func (c Client) Forget(ctx context.Context, r Retention, dryRun bool) error {
	args := []string{"forget"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	if r.KeepHourly > 0 {
		args = append(args, "--keep-hourly", fmt.Sprint(r.KeepHourly))
	}
	if r.KeepDaily > 0 {
		args = append(args, "--keep-daily", fmt.Sprint(r.KeepDaily))
	}
	if r.KeepWeekly > 0 {
		args = append(args, "--keep-weekly", fmt.Sprint(r.KeepWeekly))
	}
	if r.KeepMonthly > 0 {
		args = append(args, "--keep-monthly", fmt.Sprint(r.KeepMonthly))
	}
	_, _, err := c.run(ctx, args...)
	return err
}
func (c Client) Prune(ctx context.Context) error { _, _, err := c.run(ctx, "prune"); return err }

func (c *Client) WipePassword() {
	if c == nil {
		return
	}
	for i := range c.Password {
		c.Password[i] = 0
	}
	c.Password = nil
}
