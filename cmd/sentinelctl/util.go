package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func (a *app) requireGoVersion() error {
	b, err := os.ReadFile(filepath.Join(a.root, ".go-version"))
	if err != nil {
		return nil
	}
	need := strings.TrimSpace(string(b))
	got, err := output(a.root, nil, "go", "version")
	if err != nil {
		return err
	}
	fields := strings.Fields(got)
	if len(fields) < 3 {
		return fmt.Errorf("cannot parse Go version: %s", got)
	}
	have := strings.TrimPrefix(fields[2], "go")
	if compareVersion(have, need) < 0 {
		return fmt.Errorf("Go %s or newer is required; found %s", need, have)
	}
	return nil
}

func compareVersion(a, b string) int {
	parse := func(s string) []int {
		s = strings.TrimPrefix(s, "go")
		var core strings.Builder
		for _, r := range s {
			if (r >= '0' && r <= '9') || r == '.' {
				core.WriteRune(r)
			} else {
				break
			}
		}
		parts := strings.Split(strings.Trim(core.String(), "."), ".")
		out := make([]int, len(parts))
		for i, p := range parts {
			out[i], _ = strconv.Atoi(p)
		}
		return out
	}
	aa, bb := parse(a), parse(b)
	n := len(aa)
	if len(bb) > n {
		n = len(bb)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(aa) {
			av = aa[i]
		}
		if i < len(bb) {
			bv = bb[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			if !replaced {
				out = append(out, prefix+value)
				replaced = true
			}
			continue
		}
		out = append(out, item)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
func executableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}
func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && (st.Mode()&os.ModeCharDevice) != 0
}
func portAvailable(addr string) bool {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

func output(dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = env
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	return strings.TrimSpace(string(b)), nil
}

func stream(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = env
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func printTail(r io.ReadSeeker, n int, w io.Writer) error {
	if n == 0 {
		return nil
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return err
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	lines := bytes.Split(b, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:] {
		if _, err := fmt.Fprintln(w, string(line)); err != nil {
			return err
		}
	}
	return nil
}
