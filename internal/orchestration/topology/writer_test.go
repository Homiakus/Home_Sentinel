package topology

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	helperModeEnv = "SENTINEL_TOPOLOGY_HELPER"
	helperRootEnv = "SENTINEL_TOPOLOGY_ROOT"
)

func TestCanonicalRootAliasesAndValidation(t *testing.T) {
	if _, err := CanonicalRoot("   "); !errors.Is(err, ErrRuntimeRootRequired) {
		t.Fatalf("blank root error=%v want %v", err, ErrRuntimeRootRequired)
	}
	if _, err := CanonicalRoot("bad\x00root"); err == nil {
		t.Fatal("NUL-containing root unexpectedly succeeded")
	}

	base := t.TempDir()
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	rel, err := filepath.Rel(workingDir, base)
	if err != nil {
		t.Fatalf("relative path: %v", err)
	}
	fromRelative, err := CanonicalRoot(rel)
	if err != nil {
		t.Fatalf("canonical relative: %v", err)
	}
	fromAbsolute, err := CanonicalRoot(base)
	if err != nil {
		t.Fatalf("canonical absolute: %v", err)
	}
	if fromRelative != fromAbsolute {
		t.Fatalf("canonical aliases differ: relative=%q absolute=%q", fromRelative, fromAbsolute)
	}
}

func TestWriterGuardExcludesAndReleasesSameProcess(t *testing.T) {
	root := t.TempDir()
	first, err := AcquireWriter(root)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if first.Root() == "" {
		t.Fatal("writer root is empty")
	}
	if _, err := AcquireWriter(filepath.Join(root, ".")); !errors.Is(err, ErrWriterUnavailable) {
		_ = first.Close()
		t.Fatalf("second acquire error=%v want %v", err, ErrWriterUnavailable)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("idempotent close: %v", err)
	}
	second, err := AcquireWriter(root)
	if err != nil {
		t.Fatalf("reacquire after close: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	var nilGuard *WriterGuard
	if nilGuard.Root() != "" {
		t.Fatal("nil guard root must be empty")
	}
	if err := nilGuard.Close(); err != nil {
		t.Fatalf("nil guard close: %v", err)
	}
	if err := (&WriterGuard{}).Close(); err != nil {
		t.Fatalf("empty guard close: %v", err)
	}
}

func TestWriterGuardCrossProcessExclusion(t *testing.T) {
	root := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	cmd := exec.Command(executable, "-test.run=^TestWriterGuardHelperProcess$")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("helper stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("helper stdout: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), helperModeEnv+"=1", helperRootEnv+"="+root)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		t.Fatalf("read helper readiness: %v stderr=%s", err, stderr.String())
	}
	if strings.TrimSpace(line) != "READY" {
		_ = stdin.Close()
		_ = cmd.Wait()
		t.Fatalf("helper readiness=%q stderr=%s", line, stderr.String())
	}

	if _, err := AcquireWriter(root); !errors.Is(err, ErrWriterUnavailable) {
		_ = stdin.Close()
		_ = cmd.Wait()
		t.Fatalf("cross-process second acquire error=%v want %v", err, ErrWriterUnavailable)
	}
	if err := stdin.Close(); err != nil {
		_ = cmd.Wait()
		t.Fatalf("release helper: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper exit: %v stderr=%s", err, stderr.String())
	}

	reacquired, err := AcquireWriter(root)
	if err != nil {
		t.Fatalf("reacquire after helper exit: %v", err)
	}
	if err := reacquired.Close(); err != nil {
		t.Fatalf("close reacquired writer: %v", err)
	}
}

func TestWriterGuardHelperProcess(t *testing.T) {
	if os.Getenv(helperModeEnv) != "1" {
		return
	}
	root := os.Getenv(helperRootEnv)
	guard, err := AcquireWriter(root)
	if err != nil {
		t.Fatalf("helper acquire: %v", err)
	}
	if _, err := fmt.Fprintln(os.Stdout, "READY"); err != nil {
		_ = guard.Close()
		t.Fatalf("helper readiness: %v", err)
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		_ = guard.Close()
		t.Fatalf("helper wait: %v", err)
	}
	if err := guard.Close(); err != nil {
		t.Fatalf("helper close: %v", err)
	}
}
