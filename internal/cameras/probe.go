package cameras

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/media"
)

type MediaTest struct {
	Probe         media.ProbeResult `json:"probe"`
	Snapshot      []byte            `json:"-"`
	SnapshotBytes int               `json:"snapshot_bytes"`
	Latency       time.Duration     `json:"latency"`
}

func TestMedia(ctx context.Context, resolvedURL string, timeout time.Duration) (MediaTest, error) {
	start := time.Now()
	p, err := media.Probe(ctx, resolvedURL, timeout)
	if err != nil {
		return MediaTest{}, err
	}
	shot, err := Snapshot(ctx, resolvedURL, timeout)
	if err != nil {
		return MediaTest{}, err
	}
	return MediaTest{Probe: p, Snapshot: shot, SnapshotBytes: len(shot), Latency: time.Since(start)}, nil
}
func Snapshot(ctx context.Context, input string, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg unavailable: %w", err)
	}
	var args []string
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "video=") || strings.HasPrefix(input, "dshow:") || (runtime.GOOS == "windows" && strings.HasPrefix(input, "uvc://")) {
		target := strings.TrimPrefix(input, "dshow:")
		target = strings.TrimPrefix(target, "uvc://")
		if !strings.HasPrefix(target, "video=") {
			target = "video=" + target
		}
		args = []string{"-hide_banner", "-loglevel", "error", "-f", "dshow", "-i", target, "-frames:v", "1", "-f", "image2pipe", "-vcodec", "mjpeg", "pipe:1"}
	} else if strings.HasPrefix(input, "/dev/video") || strings.HasPrefix(input, "v4l2:") {
		target := strings.TrimPrefix(input, "v4l2:")
		args = []string{"-hide_banner", "-loglevel", "error", "-f", "v4l2", "-i", target, "-frames:v", "1", "-f", "image2pipe", "-vcodec", "mjpeg", "pipe:1"}
	} else {
		args = []string{"-hide_banner", "-loglevel", "error", "-rtsp_transport", "tcp", "-i", input, "-frames:v", "1", "-f", "image2pipe", "-vcodec", "mjpeg", "pipe:1"}
	}
	cmd := exec.CommandContext(ctx, path, args...)
	var out cappedBuffer
	out.max = 16 << 20
	cmd.Stdout = &out
	var stderr bytes.Buffer
	stderr.Grow(4096)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("snapshot extraction failed")
	}
	if len(out.buf) == 0 {
		return nil, errors.New("snapshot is empty")
	}
	return out.buf, nil
}

type cappedBuffer struct {
	buf []byte
	max int
}

func (w *cappedBuffer) Write(p []byte) (int, error) {
	if len(w.buf)+len(p) > w.max {
		return 0, errors.New("snapshot exceeds size limit")
	}
	w.buf = append(w.buf, p...)
	return len(p), nil
}
