package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Homiakus/Home_Sentinel/internal/security/firewall"
	"os"
)

func main() {
	var in, out, matrix string
	flag.StringVar(&in, "policy", "", "policy JSON")
	flag.StringVar(&out, "out", "-", "nftables output path or -")
	flag.StringVar(&matrix, "matrix", "", "optional connection matrix JSON output")
	flag.Parse()
	if in == "" {
		fatal("-policy is required")
	}
	b, err := os.ReadFile(in)
	if err != nil {
		fatal(err.Error())
	}
	p, err := firewall.ParseJSON(b)
	if err != nil {
		fatal(err.Error())
	}
	nft, err := firewall.RenderNFT(p)
	if err != nil {
		fatal(err.Error())
	}
	if out == "-" {
		fmt.Print(nft)
	} else if err := os.WriteFile(out, []byte(nft), 0640); err != nil {
		fatal(err.Error())
	}
	if matrix != "" {
		m, err := firewall.Matrix(p)
		if err != nil {
			fatal(err.Error())
		}
		j, _ := json.MarshalIndent(m, "", "  ")
		if err := os.WriteFile(matrix, append(j, '\n'), 0640); err != nil {
			fatal(err.Error())
		}
	}
}
func fatal(s string) { fmt.Fprintln(os.Stderr, "sentinel-firewall:", s); os.Exit(1) }
