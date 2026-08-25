package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Homiakus/Home_Sentinel/internal/release"
)

func main() {
	in := flag.String("input", "", "qualification JSON")
	out := flag.String("out", "", "markdown report path; stdout when empty")
	flag.Parse()
	if *in == "" {
		fatal("--input is required")
	}
	f, err := os.Open(*in)
	if err != nil {
		fatal(err.Error())
	}
	defer f.Close()
	q, err := release.DecodeQualification(f)
	if err != nil {
		fatal(err.Error())
	}
	var w = os.Stdout
	if *out != "" {
		x, e := os.Create(*out)
		if e != nil {
			fatal(e.Error())
		}
		defer x.Close()
		w = x
	}
	if err := release.WriteQualificationMarkdown(w, q); err != nil {
		fatal(err.Error())
	}
	ok, blockers := q.Releasable()
	if !ok {
		fmt.Fprintln(os.Stderr, "release blocked:", blockers)
		os.Exit(2)
	}
}
func fatal(s string) { fmt.Fprintln(os.Stderr, "sentinel-qualify:", s); os.Exit(1) }
