package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func (a *app) runForeground() error {
	if err := a.setup(); err != nil {
		return err
	}
	if _, err := os.Stat(a.binary); err != nil {
		if err := a.build(); err != nil {
			return err
		}
	}
	return stream(a.root, a.runtimeEnv(), a.binary, "serve")
}

func (a *app) start() error {
	if err := a.setup(); err != nil {
		return err
	}
	a.cleanupStalePID()
	if pid, ok := a.runningPID(); ok {
		fmt.Printf("Home Sentinel is already running (PID %d).\n", pid)
		return nil
	}
	if _, err := os.Stat(a.binary); err != nil {
		if err := a.build(); err != nil {
			return err
		}
	}
	logf, err := os.OpenFile(a.logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd := exec.Command(a.binary, "serve")
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = a.root, a.runtimeEnv(), logf, logf
	prepareDetached(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	if err := os.WriteFile(a.pidFile, []byte(strconv.Itoa(pid)+"\n"), 0600); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	_ = cmd.Process.Release()
	time.Sleep(900 * time.Millisecond)
	if !processAlive(pid) {
		_ = os.Remove(a.pidFile)
		return fmt.Errorf("Home Sentinel exited during startup; inspect %s", a.logFile)
	}
	fmt.Printf("Started (PID %d). Logs: %s\n", pid, a.logFile)
	return nil
}

func (a *app) stop() error {
	a.cleanupStalePID()
	pid, ok := a.runningPID()
	if !ok {
		fmt.Println("Home Sentinel is not running.")
		return nil
	}
	fmt.Printf("Stopping Home Sentinel (PID %d)...\n", pid)
	if err := terminateProcess(pid); err != nil {
		return err
	}
	deadline := time.Now().Add(15 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
	if processAlive(pid) {
		if err := killProcess(pid); err != nil {
			return err
		}
	}
	_ = os.Remove(a.pidFile)
	fmt.Println("Stopped.")
	return nil
}

func (a *app) restartHost() error {
	if err := a.stop(); err != nil {
		return err
	}
	return a.start()
}

func (a *app) runningPID() (int, bool) {
	b, err := os.ReadFile(a.pidFile)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 || !processAlive(pid) {
		return 0, false
	}
	return pid, true
}

func (a *app) cleanupStalePID() {
	if _, ok := a.runningPID(); !ok {
		_ = os.Remove(a.pidFile)
	}
}

func (a *app) status() error {
	a.cleanupStalePID()
	fmt.Println("Home Sentinel")
	if pid, ok := a.runningPID(); ok {
		fmt.Printf(" state:   running\n pid:     %d\n", pid)
	} else {
		fmt.Println(" state:   stopped")
	}
	if _, err := os.Stat(a.binary); err == nil {
		fmt.Println(" binary: ", a.binary)
		_ = stream(a.root, nil, a.binary, "version")
	} else {
		fmt.Println(" binary:  not built")
	}
	fmt.Printf(" config:  %s\n data:    %s\n log:     %s\n", a.configFile, a.dataDir, a.logFile)
	if portAvailable("127.0.0.1:8080") {
		fmt.Println(" port:    127.0.0.1:8080 available")
	} else {
		fmt.Println(" port:    127.0.0.1:8080 in use")
	}
	return nil
}

func (a *app) logs(args []string) error {
	n := 100
	if len(args) > 1 {
		return fmt.Errorf("logs accepts at most one line count")
	}
	if len(args) == 1 {
		v, err := strconv.Atoi(args[0])
		if err != nil || v < 0 {
			return fmt.Errorf("logs line count must be non-negative")
		}
		n = v
	}
	if err := a.ensureDirs(); err != nil {
		return err
	}
	f, err := os.OpenFile(a.logFile, os.O_CREATE|os.O_RDONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := printTail(f, n, os.Stdout); err != nil {
		return err
	}
	offset, _ := f.Seek(0, io.SeekEnd)
	for {
		time.Sleep(500 * time.Millisecond)
		stat, err := f.Stat()
		if err != nil {
			return err
		}
		if stat.Size() < offset {
			offset = 0
		}
		if stat.Size() > offset {
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				return err
			}
			if _, err := io.Copy(os.Stdout, f); err != nil {
				return err
			}
			offset = stat.Size()
		}
	}
}
