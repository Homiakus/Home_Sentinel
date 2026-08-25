package hardware

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ProbeVideo(ctx context.Context, r Runner) VideoInfo {
	v := VideoInfo{}
	entries, _ := filepath.Glob("/dev/dri/renderD*")
	sort.Strings(entries)
	v.DRIDevices = entries
	if len(entries) == 0 {
		v.VAAPI = Capability{Reason: "no /dev/dri/renderD* devices"}
	} else if r == nil {
		v.VAAPI = Capability{Reason: "VAAPI command probe not configured"}
	} else {
		out, err := r.Run(ctx, "vainfo")
		if err != nil {
			v.VAAPI = Capability{Reason: err.Error()}
		} else {
			raw := string(out)
			details := make([]string, 0)
			for _, line := range strings.Split(raw, "\n") {
				line = strings.TrimSpace(line)
				if strings.Contains(line, "VAProfile") && strings.Contains(line, "VAEntrypointVLD") {
					details = append(details, line)
					if len(details) >= 12 {
						break
					}
				}
			}
			v.VAAPI = Capability{Available: true, Details: details}
		}
	}
	if r != nil {
		if out, err := r.Run(ctx, "nvidia-smi", "--query-gpu=name,memory.total,driver_version", "--format=csv,noheader,nounits"); err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				parts := strings.Split(line, ",")
				if len(parts) < 3 {
					continue
				}
				gpu := NVIDIAGPU{Name: strings.TrimSpace(parts[0]), Driver: strings.TrimSpace(parts[2])}
				fmtSscan(strings.TrimSpace(parts[1]), &gpu.VRAMMiB)
				v.NVIDIA = append(v.NVIDIA, gpu)
			}
		}
	}
	return v
}
func fmtSscan(s string, dst *int) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	*dst = n
}
func DRIDeviceAccessible(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
