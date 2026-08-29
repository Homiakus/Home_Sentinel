//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

func prepareDetached(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
func processAlive(pid int) bool {
	p, e := os.FindProcess(pid)
	return e == nil && p.Signal(syscall.Signal(0)) == nil
}
func terminateProcess(pid int) error {
	p, e := os.FindProcess(pid)
	if e != nil {
		return e
	}
	return p.Signal(syscall.SIGTERM)
}
func killProcess(pid int) error {
	p, e := os.FindProcess(pid)
	if e != nil {
		return e
	}
	return p.Kill()
}
func isRoot() bool { return os.Geteuid() == 0 }
