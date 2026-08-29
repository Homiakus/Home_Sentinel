package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

func main() {
	a, err := newApp()
	if err == nil {
		err = a.run(os.Args[1:])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "sentinelctl:", err)
		os.Exit(1)
	}
}

func (a *app) run(args []string) error {
	if len(args) == 0 {
		if isTerminal(os.Stdin) {
			return a.menu()
		}
		a.usage()
		return nil
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "help", "-h", "--help":
		a.usage()
		return nil
	case "menu":
		return a.menu()
	case "setup":
		return a.setup()
	case "configure", "config":
		return a.configure()
	case "doctor":
		return a.doctor()
	case "build":
		return a.build()
	case "image":
		return a.image()
	case "check", "test":
		return a.check()
	case "run":
		return a.runForeground()
	case "start":
		return a.start()
	case "stop":
		return a.stop()
	case "restart":
		return a.restartHost()
	case "status":
		return a.status()
	case "logs":
		return a.logs(rest)
	case "update":
		return a.update()
	case "stack-config":
		return a.stackConfig()
	case "stack-up":
		return a.stackUp()
	case "stack-down":
		return a.compose("down")
	case "stack-restart":
		if err := a.stackConfig(); err != nil {
			return err
		}
		return a.compose("restart")
	case "stack-status":
		return a.compose("ps")
	case "stack-logs":
		return a.compose("logs", "-f", "--tail", "100")
	case "stack-pull":
		if err := a.stackConfig(); err != nil {
			return err
		}
		if err := a.compose("pull"); err != nil {
			return err
		}
		return a.compose("up", "-d")
	case "service-install":
		return a.serviceInstall()
	case "service-enable":
		return a.serviceEnable()
	case "service-disable":
		return a.serviceDisable()
	case "service-status":
		return a.serviceStatus()
	case "service-remove":
		return a.serviceRemove()
	default:
		return fmt.Errorf("unknown command %q (try --help)", cmd)
	}
}

func (a *app) usage() {
	fmt.Print(`Home Sentinel cross-platform manager

Usage:
  go run ./cmd/sentinelctl <command>
  sentinelctl <command>

Host mode:
  menu, setup, configure, doctor, build, image, check,
  run, start, stop, restart, status, logs [N], update

Docker Compose:
  stack-config, stack-up, stack-down, stack-restart,
  stack-status, stack-logs, stack-pull

Linux service integration:
  service-install, service-enable, service-disable,
  service-status, service-remove
`)
}

func (a *app) menu() error {
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\nHome Sentinel\n 1) Setup\n 2) Doctor\n 3) Build\n 4) Start\n 5) Stop\n 6) Restart\n 7) Status\n 8) Configure\n 9) Check/tests\n10) Stack up\n11) Stack status\n12) Update\n 0) Exit\n> ")
		line, err := in.ReadString('\n')
		if err != nil {
			return err
		}
		var opErr error
		switch strings.TrimSpace(line) {
		case "1":
			opErr = a.setup()
		case "2":
			opErr = a.doctor()
		case "3":
			opErr = a.build()
		case "4":
			opErr = a.start()
		case "5":
			opErr = a.stop()
		case "6":
			opErr = a.restartHost()
		case "7":
			opErr = a.status()
		case "8":
			opErr = a.configure()
		case "9":
			opErr = a.check()
		case "10":
			opErr = a.stackUp()
		case "11":
			opErr = a.compose("ps")
		case "12":
			opErr = a.update()
		case "0":
			return nil
		default:
			opErr = errors.New("unknown menu item")
		}
		if opErr != nil {
			fmt.Fprintln(os.Stderr, "error:", opErr)
		}
	}
}
