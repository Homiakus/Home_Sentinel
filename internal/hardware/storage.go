package hardware

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func ProbeStorage() []MountInfo {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []MountInfo
	s := bufio.NewScanner(f)
	seen := map[string]bool{}
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 3 {
			continue
		}
		dev, mount, fs := unescapeMount(fields[0]), unescapeMount(fields[1]), fields[2]
		if seen[mount] || skipFilesystem(fs) {
			continue
		}
		seen[mount] = true
		total, free, ok := getDiskSpace(mount)
		if !ok {
			continue
		}
		m := MountInfo{Device: dev, MountPoint: mount, Filesystem: fs, Total: total, Free: free}
		if strings.HasPrefix(dev, "/dev/") {
			if rot, ok := rotational(dev); ok {
				m.Rotational = &rot
			}
		}
		out = append(out, m)
	}
	return out
}
func skipFilesystem(fs string) bool {
	switch fs {
	case "proc", "sysfs", "tmpfs", "devtmpfs", "devpts", "cgroup", "cgroup2", "overlay", "squashfs", "securityfs", "tracefs", "debugfs", "pstore", "mqueue", "hugetlbfs", "fusectl":
		return true
	}
	return false
}
func unescapeMount(s string) string {
	r := strings.NewReplacer("\\040", " ", "\\011", "\t", "\\012", "\n", "\\134", "\\")
	return r.Replace(s)
}
func rotational(dev string) (bool, bool) {
	base := filepath.Base(dev)
	root := base
	if strings.HasPrefix(base, "nvme") && strings.Contains(base, "p") {
		root = strings.Split(base, "p")[0]
	} else {
		root = strings.TrimRight(base, "0123456789")
	}
	b, err := os.ReadFile(filepath.Join("/sys/class/block", root, "queue/rotational"))
	if err != nil {
		return false, false
	}
	return strings.TrimSpace(string(b)) == "1", true
}
