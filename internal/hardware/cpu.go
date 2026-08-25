package hardware

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func ProbeCPU() CPUInfo {
	c := CPUInfo{LogicalCores: runtime.NumCPU(), Flags: map[string]bool{}}
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return c
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if c.Model == "" && (k == "model name" || k == "Hardware" || k == "Processor") {
			c.Model = v
		}
		if k == "flags" || k == "Features" {
			for _, flag := range strings.Fields(v) {
				c.Flags[strings.ToLower(flag)] = true
			}
		}
	}
	return c
}
func ProbeMemory() MemoryInfo {
	m := MemoryInfo{}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return m
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		bytes := n * 1024
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			m.Total = bytes
		case "MemAvailable":
			m.Available = bytes
		}
	}
	return m
}
