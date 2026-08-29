package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func (a *app) serviceInstall() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("systemd integration is Linux-only; use start/stop on %s", runtime.GOOS)
	}
	if err := a.setup(); err != nil {
		return err
	}
	if _, err := os.Stat(a.binary); err != nil {
		if err := a.build(); err != nil {
			return err
		}
	}
	user := envOr("USER", "")
	if user == "" {
		if s, err := output(a.root, nil, "id", "-un"); err == nil {
			user = s
		}
	}
	unit := fmt.Sprintf("[Unit]\nDescription=Home Sentinel local security/automation service\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nType=simple\nUser=%s\nWorkingDirectory=%s\nEnvironment=SENTINEL_CONFIG=%s\nEnvironment=SENTINEL_DB_PATH=%s\nEnvironment=SENTINEL_LISTEN=127.0.0.1:8080\nEnvironment=SENTINEL_FRIGATE_CREDENTIALS_DIR=%s\nExecStart=%s serve\nRestart=on-failure\nRestartSec=3s\nTimeoutStopSec=20s\nNoNewPrivileges=true\nPrivateTmp=true\nUMask=0077\n\n[Install]\nWantedBy=multi-user.target\n", user, a.root, a.configFile, filepath.Join(a.dataDir, "sentinel.db"), filepath.Join(a.varDir, "frigate-secrets"), a.binary)
	tmp := filepath.Join(a.runDir, serviceName+".service")
	if err := os.WriteFile(tmp, []byte(unit), 0600); err != nil {
		return err
	}
	if err := sudo("install", "-m", "0644", tmp, "/etc/systemd/system/"+serviceName+".service"); err != nil {
		return err
	}
	return sudo("systemctl", "daemon-reload")
}

func (a *app) serviceEnable() error  { return linuxService("enable", "--now", serviceName+".service") }
func (a *app) serviceDisable() error { return linuxService("disable", "--now", serviceName+".service") }
func (a *app) serviceStatus() error {
	if runtime.GOOS != "linux" {
		return errors.New("systemd integration requires Linux")
	}
	return stream(a.root, nil, "systemctl", "status", serviceName+".service", "--no-pager")
}
func (a *app) serviceRemove() error {
	if runtime.GOOS != "linux" {
		return errors.New("systemd integration requires Linux")
	}
	_ = sudo("systemctl", "disable", "--now", serviceName+".service")
	if err := sudo("rm", "-f", "/etc/systemd/system/"+serviceName+".service"); err != nil {
		return err
	}
	return sudo("systemctl", "daemon-reload")
}

func linuxService(args ...string) error {
	if runtime.GOOS != "linux" {
		return errors.New("systemd integration requires Linux")
	}
	return sudo(append([]string{"systemctl"}, args...)...)
}

func sudo(args ...string) error {
	if isRoot() {
		return stream("", nil, args[0], args[1:]...)
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return errors.New("root privileges required and sudo not found")
	}
	return stream("", nil, "sudo", args...)
}
