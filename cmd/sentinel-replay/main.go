package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/events"
	"github.com/Homiakus/Home_Sentinel/internal/incidents"
	"io"
	"os"
	"time"
)

func main() {
	window := flag.Duration("window", 45*time.Second, "correlation window")
	flag.Parse()
	var r io.Reader = os.Stdin
	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fatal(err)
		}
		defer f.Close()
		r = f
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	var in []events.Envelope
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var e events.Envelope
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			fatal(err)
		}
		in = append(in, e)
	}
	if err := scanner.Err(); err != nil {
		fatal(err)
	}
	seq := 0
	idf := func(string) (domain.ID, error) { seq++; return domain.ID(fmt.Sprintf("inc_%026d", seq)), nil }
	out, err := incidents.Correlate(in, *window, idf)
	if err != nil && err != incidents.ErrNoEvents {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{"incidents": out}); err != nil {
		fatal(err)
	}
}
func fatal(err error) { fmt.Fprintln(os.Stderr, "sentinel-replay:", err); os.Exit(1) }
