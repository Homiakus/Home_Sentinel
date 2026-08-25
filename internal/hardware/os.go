package hardware

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func ProbeOS() OSInfo {
	o := OSInfo{Arch: runtime.GOARCH}
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			k, v, ok := strings.Cut(s.Text(), "=")
			if !ok {
				continue
			}
			v = strings.Trim(strings.TrimSpace(v), "\"")
			switch k {
			case "ID":
				o.ID = v
			case "VERSION_ID":
				o.Version = v
			case "PRETTY_NAME":
				o.PrettyName = v
			}
		}
	}
	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		o.Kernel = strings.TrimSpace(string(b))
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		o.Container = true
	} else if b, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		raw := string(b)
		o.Container = strings.Contains(raw, "docker") || strings.Contains(raw, "kubepods") || strings.Contains(raw, "containerd")
	}
	for _, p := range []string{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory/memory.limit_in_bytes"} {
		if b, err := os.ReadFile(p); err == nil {
			v := strings.TrimSpace(string(b))
			if v != "max" {
				if n, e := strconv.ParseUint(v, 10, 64); e == nil && n < (1<<62) {
					o.CgroupMemoryLimit = n
					break
				}
			}
		}
	}
	return o
}
