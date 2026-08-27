package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Homiakus/Home_Sentinel/internal/engloop"
)

func main() {
	root := flag.String("root", ".", "repository root")
	jsonOut := flag.Bool("json", false, "write JSON report")
	flag.Parse()

	report, err := engloop.VerifySupplyChain(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sentinel-supplychain:", err)
		os.Exit(1)
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, "sentinel-supplychain:", err)
			os.Exit(1)
		}
	} else {
		for _, finding := range report.Findings {
			path := ""
			if finding.Path != "" {
				path = " [" + finding.Path + "]"
			}
			fmt.Printf("%s %s%s: %s\n", finding.Severity, finding.ID, path, finding.Message)
		}
	}
	if report.HasBlockers() {
		os.Exit(3)
	}
}
