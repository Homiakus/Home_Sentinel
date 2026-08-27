package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/engloop"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var exitErr exitError
		if errors.As(err, &exitErr) {
			fmt.Fprintln(os.Stderr, exitErr.Error())
			os.Exit(exitErr.code)
		}
		fmt.Fprintln(os.Stderr, "sentinel-engloop:", err)
		os.Exit(1)
	}
}

type exitError struct {
	code int
	msg  string
}

func (e exitError) Error() string { return e.msg }

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stderr)
		return exitError{code: 2, msg: "command is required"}
	}
	switch args[0] {
	case "reconcile":
		return runReconcile(args[1:], stdout, stderr)
	case "packet":
		return runPacket(args[1:], stdout, stderr)
	case "gates":
		return runGates(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		usage(stdout)
		return nil
	default:
		usage(stderr)
		return exitError{code: 2, msg: fmt.Sprintf("unknown command %q", args[0])}
	}
}

func runReconcile(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	jsonOut := fs.Bool("json", false, "write JSON report")
	strict := fs.Bool("strict", false, "fail when any WARNING or BLOCKER is found")
	if err := fs.Parse(args); err != nil {
		return exitError{code: 2, msg: err.Error()}
	}
	report, err := engloop.Reconcile(*root)
	if err != nil {
		return err
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
	} else {
		for _, finding := range report.Findings {
			path := ""
			if finding.Path != "" {
				path = " [" + finding.Path + "]"
			}
			fmt.Fprintf(stdout, "%s %s%s: %s\n", finding.Severity, finding.ID, path, finding.Message)
		}
	}
	if report.HasBlockers() {
		return exitError{code: 3, msg: "reconciliation found blocking contradictions"}
	}
	if *strict {
		for _, finding := range report.Findings {
			if finding.Severity == engloop.SeverityWarning {
				return exitError{code: 4, msg: "strict reconciliation found warnings"}
			}
		}
	}
	return nil
}

func runPacket(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("packet", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", "", "work packet JSON; stdin when empty")
	if err := fs.Parse(args); err != nil {
		return exitError{code: 2, msg: err.Error()}
	}
	var r io.Reader = os.Stdin
	if *file != "" {
		f, err := os.Open(*file)
		if err != nil {
			return fmt.Errorf("open work packet: %w", err)
		}
		defer f.Close()
		r = f
	}
	packet, err := engloop.DecodeWorkPacket(r)
	if err != nil {
		return exitError{code: 3, msg: err.Error()}
	}
	fmt.Fprintf(stdout, "VALID %s risk=%s gates=%d invariants=%d\n", packet.PlanItem, packet.RiskClass, len(packet.RequiredGates), len(packet.Invariants))
	return nil
}

func runGates(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("gates", flag.ContinueOnError)
	fs.SetOutput(stderr)
	changedFile := fs.String("changed-file", "", "newline-delimited changed paths; stdin when empty")
	riskFlag := fs.String("risk", "auto", "auto|low|medium|high|critical")
	if err := fs.Parse(args); err != nil {
		return exitError{code: 2, msg: err.Error()}
	}

	var r io.Reader = os.Stdin
	if *changedFile != "" {
		f, err := os.Open(*changedFile)
		if err != nil {
			return fmt.Errorf("open changed-file: %w", err)
		}
		defer f.Close()
		r = f
	}
	paths, err := readLines(r)
	if err != nil {
		return fmt.Errorf("read changed paths: %w", err)
	}
	if len(paths) == 0 {
		return exitError{code: 2, msg: "no changed paths supplied"}
	}

	risk := engloop.ClassifyPaths(paths)
	if strings.ToLower(strings.TrimSpace(*riskFlag)) != "auto" {
		var ok bool
		risk, ok = parseRisk(*riskFlag)
		if !ok {
			return exitError{code: 2, msg: fmt.Sprintf("invalid --risk %q", *riskFlag)}
		}
	}
	result := struct {
		Risk            engloop.RiskClass `json:"risk"`
		Gates           []engloop.Gate    `json:"gates"`
		MutationTargets []string          `json:"mutation_targets"`
	}{
		Risk:            risk,
		Gates:           engloop.GatePlan(paths, risk),
		MutationTargets: engloop.MutationTargets(paths),
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("write gate plan: %w", err)
	}
	return nil
}

func readLines(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	paths := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			paths = append(paths, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return paths, nil
}

func parseRisk(s string) (engloop.RiskClass, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case string(engloop.RiskLow):
		return engloop.RiskLow, true
	case string(engloop.RiskMedium):
		return engloop.RiskMedium, true
	case string(engloop.RiskHigh):
		return engloop.RiskHigh, true
	case string(engloop.RiskCritical):
		return engloop.RiskCritical, true
	default:
		return "", false
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: sentinel-engloop <reconcile|packet|gates> [flags]")
	fmt.Fprintln(w, "  reconcile  compare recorded roadmap status with observed checkout")
	fmt.Fprintln(w, "  packet     validate a machine-readable Work Packet")
	fmt.Fprintln(w, "  gates      derive risk, gates and mutation targets from changed paths")
}
