package uvc

import (
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Device struct {
	Path  string `json:"path"`
	Name  string `json:"name,omitempty"`
	Modes []Mode `json:"modes,omitempty"`
}

type Mode struct {
	PixelFormat string  `json:"pixel_format"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FPS         float64 `json:"fps"`
}

func Discover(ctx context.Context) []Device {
	if runtime.GOOS == "windows" {
		return discoverWindows(ctx)
	}
	return discoverLinux(ctx)
}

func discoverLinux(ctx context.Context) []Device {
	paths, _ := filepath.Glob("/dev/video*")
	sort.Strings(paths)
	out := make([]Device, 0, len(paths))
	tool, toolErr := exec.LookPath("v4l2-ctl")
	for _, p := range paths {
		d := Device{Path: p, Name: filepath.Base(p)}
		if toolErr == nil {
			cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			info, err := exec.CommandContext(cctx, tool, "--device", p, "--info").CombinedOutput()
			cancel()
			if err == nil {
				if card := parseCardName(string(info)); card != "" {
					d.Name = card
				}
			}
			cctx, cancel = context.WithTimeout(ctx, 3*time.Second)
			formats, err := exec.CommandContext(cctx, tool, "--device", p, "--list-formats-ext").CombinedOutput()
			cancel()
			if err == nil {
				d.Modes = parseModes(string(formats))
			}
		}
		out = append(out, d)
	}
	return out
}

func discoverWindows(ctx context.Context) []Device {
	var out []Device
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err == nil {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		raw, err := exec.CommandContext(cctx, ffmpeg, "-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy").CombinedOutput()
		cancel()
		if err != nil || len(raw) > 0 {
			out = parseDShowDevices(string(raw))
			for i := range out {
				cctx, cancel := context.WithTimeout(ctx, 4*time.Second)
				modesRaw, _ := exec.CommandContext(cctx, ffmpeg, "-hide_banner", "-list_options", "true", "-f", "dshow", "-i", "video="+out[i].Name).CombinedOutput()
				cancel()
				if len(modesRaw) > 0 {
					out[i].Modes = parseDShowModes(string(modesRaw))
				}
			}
		}
	}
	if len(out) == 0 {
		out = discoverWindowsPnP(ctx)
	}
	return out
}

var dshowDeviceRE = regexp.MustCompile(`"(.*?)"\s+\(video\)`)

func parseDShowDevices(raw string) []Device {
	var out []Device
	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		m := dshowDeviceRE.FindStringSubmatch(line)
		if len(m) == 2 {
			name := strings.TrimSpace(m[1])
			if name != "" && !seen[name] {
				seen[name] = true
				out = append(out, Device{
					Path: "video=" + name,
					Name: name,
				})
			}
		}
	}
	return out
}

var dshowModeRE = regexp.MustCompile(`(?:pixel_format|vcodec)=(\w+).*?(?:max\s+s=(\d+)x(\d+)\s+fps=([0-9]+(?:\.[0-9]+)?)|min\s+s=(\d+)x(\d+)\s+fps=.*?max\s+s=(\d+)x(\d+)\s+fps=([0-9]+(?:\.[0-9]+)?)|s=(\d+)x(\d+)\s+fps=([0-9]+(?:\.[0-9]+)?))`)

func parseDShowModes(raw string) []Mode {
	var out []Mode
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "pixel_format") && !strings.Contains(line, "vcodec") {
			continue
		}
		// Format sample: pixel_format=yuyv422  min s=640x480 fps=1 max s=640x480 fps=30
		pixFmt := ""
		if idx := strings.Index(line, "pixel_format="); idx != -1 {
			sub := line[idx+len("pixel_format="):]
			pixFmt = strings.Fields(sub)[0]
		} else if idx := strings.Index(line, "vcodec="); idx != -1 {
			sub := line[idx+len("vcodec="):]
			pixFmt = strings.Fields(sub)[0]
		}
		var width, height int
		var fps float64
		// Try max s=WxH
		if idx := strings.Index(line, "max s="); idx != -1 {
			sub := line[idx+len("max s="):]
			parts := strings.Fields(sub)
			if len(parts) >= 1 {
				dims := strings.Split(parts[0], "x")
				if len(dims) == 2 {
					width, _ = strconv.Atoi(dims[0])
					height, _ = strconv.Atoi(dims[1])
				}
			}
			if fidx := strings.Index(line, "max s="); fidx != -1 {
				afterMax := line[fidx:]
				if fpsIdx := strings.Index(afterMax, "fps="); fpsIdx != -1 {
					fpsStr := strings.Fields(afterMax[fpsIdx+len("fps="):])[0]
					fps, _ = strconv.ParseFloat(fpsStr, 64)
				}
			}
		} else if idx := strings.Index(line, "s="); idx != -1 {
			sub := line[idx+len("s="):]
			parts := strings.Fields(sub)
			if len(parts) >= 1 {
				dims := strings.Split(parts[0], "x")
				if len(dims) == 2 {
					width, _ = strconv.Atoi(dims[0])
					height, _ = strconv.Atoi(dims[1])
				}
			}
			if fpsIdx := strings.Index(line, "fps="); fpsIdx != -1 {
				fpsStr := strings.Fields(line[fpsIdx+len("fps="):])[0]
				fps, _ = strconv.ParseFloat(fpsStr, 64)
			}
		}
		if width > 0 && height > 0 {
			if fps == 0 {
				fps = 30
			}
			out = append(out, Mode{
				PixelFormat: pixFmt,
				Width:       width,
				Height:      height,
				FPS:         fps,
			})
		}
	}
	return dedupeModes(out)
}

func discoverWindowsPnP(ctx context.Context) []Device {
	cctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "powershell", "-NoProfile", "-NonInteractive", "-Command",
		`Get-CimInstance Win32_PnPEntity | Where-Object { $_.PNPClass -in @('Camera', 'Image') } | Select-Object -ExpandProperty Name`)
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}
	var out []Device
	for _, line := range strings.Split(string(outBytes), "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			out = append(out, Device{
				Path: "video=" + name,
				Name: name,
			})
		}
	}
	return out
}

func FFmpegInput(device string) []string {
	device = strings.TrimSpace(device)
	if runtime.GOOS == "windows" || strings.HasPrefix(device, "video=") || strings.HasPrefix(device, "dshow:") {
		name := strings.TrimPrefix(device, "dshow:")
		if !strings.HasPrefix(name, "video=") {
			name = "video=" + name
		}
		return []string{"-f", "dshow", "-i", name}
	}
	dev := strings.TrimPrefix(device, "v4l2:")
	return []string{"-f", "v4l2", "-i", dev}
}

var formatRE = regexp.MustCompile(`'([^']+)'`)
var sizeRE = regexp.MustCompile(`Size:\s+Discrete\s+(\d+)x(\d+)`)
var fpsRE = regexp.MustCompile(`\(([0-9]+(?:\.[0-9]+)?)\s+fps\)`)

func parseModes(raw string) []Mode {
	var out []Mode
	format := ""
	width, height := 0, 0
	hadInterval := false
	flushSize := func() {
		if width > 0 && height > 0 && !hadInterval {
			out = append(out, Mode{PixelFormat: format, Width: width, Height: height, FPS: 30})
		}
		width, height, hadInterval = 0, 0, false
	}
	for _, line := range strings.Split(raw, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") {
			flushSize()
			if m := formatRE.FindStringSubmatch(trim); len(m) == 2 {
				format = m[1]
			}
			continue
		}
		if m := sizeRE.FindStringSubmatch(trim); len(m) == 3 {
			flushSize()
			width, _ = strconv.Atoi(m[1])
			height, _ = strconv.Atoi(m[2])
			continue
		}
		if width > 0 && height > 0 {
			if m := fpsRE.FindStringSubmatch(trim); len(m) == 2 {
				fps, _ := strconv.ParseFloat(m[1], 64)
				out = append(out, Mode{PixelFormat: format, Width: width, Height: height, FPS: fps})
				hadInterval = true
			}
		}
	}
	flushSize()
	return dedupeModes(out)
}

func dedupeModes(in []Mode) []Mode {
	seen := map[string]bool{}
	out := make([]Mode, 0, len(in))
	for _, m := range in {
		key := m.PixelFormat + "/" + strconv.Itoa(m.Width) + "x" + strconv.Itoa(m.Height) + "/" + strconv.FormatFloat(m.FPS, 'f', 1, 64)
		if !seen[key] {
			seen[key] = true
			out = append(out, m)
		}
	}
	return out
}

func parseCardName(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "Card type") {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
