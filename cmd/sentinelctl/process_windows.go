//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

const (
	createNewProcessGroup          = 0x00000200
	detachedProcess                = 0x00000008
	processQueryLimitedInformation = 0x1000
)

func prepareDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | detachedProcess, HideWindow: true}
}
func processAlive(pid int) bool {
	h, e := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if e != nil {
		return false
	}
	_ = syscall.CloseHandle(h)
	return true
}
func terminateProcess(pid int) error {
	p, e := os.FindProcess(pid)
	if e != nil {
		return e
	}
	return p.Kill()
}
func killProcess(pid int) error { return terminateProcess(pid) }
func isRoot() bool              { return false }
