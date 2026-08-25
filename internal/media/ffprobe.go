package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type ProbeResult struct {
	Format       string        `json:"format,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
	Bitrate      int64         `json:"bitrate,omitempty"`
	Video        []VideoStream `json:"video"`
	Audio        []AudioStream `json:"audio"`
	ProbeLatency time.Duration `json:"probe_latency"`
}
type VideoStream struct {
	Index   int     `json:"index"`
	Codec   string  `json:"codec"`
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	FPS     float64 `json:"fps"`
	Bitrate int64   `json:"bitrate,omitempty"`
}
type AudioStream struct {
	Index      int    `json:"index"`
	Codec      string `json:"codec"`
	SampleRate int    `json:"sample_rate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
}
type ffDoc struct {
	Streams []struct {
		Index        int    `json:"index"`
		CodecName    string `json:"codec_name"`
		CodecType    string `json:"codec_type"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"`
		RFrameRate   string `json:"r_frame_rate"`
		BitRate      string `json:"bit_rate"`
		SampleRate   string `json:"sample_rate"`
		Channels     int    `json:"channels"`
	} `json:"streams"`
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
}

func Probe(ctx context.Context, input string, timeout time.Duration) (ProbeResult, error) {
	if strings.TrimSpace(input) == "" {
		return ProbeResult{}, errors.New("media input required")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	path, err := exec.LookPath("ffprobe")
	if err != nil {
		return ProbeResult{}, fmt.Errorf("ffprobe unavailable: %w", err)
	}
	started := time.Now()
	cmd := exec.CommandContext(ctx, path, "-v", "error", "-rtsp_transport", "tcp", "-show_streams", "-show_format", "-of", "json", input)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return ProbeResult{}, fmt.Errorf("ffprobe timeout: %w", ctx.Err())
		}
		return ProbeResult{}, fmt.Errorf("ffprobe failed: %w", err)
	}
	var d ffDoc
	if err := json.Unmarshal(out, &d); err != nil {
		return ProbeResult{}, fmt.Errorf("decode ffprobe JSON: %w", err)
	}
	r := ProbeResult{Format: d.Format.FormatName, ProbeLatency: time.Since(started)}
	if seconds, e := strconv.ParseFloat(d.Format.Duration, 64); e == nil {
		r.Duration = time.Duration(seconds * float64(time.Second))
	}
	r.Bitrate, _ = strconv.ParseInt(d.Format.BitRate, 10, 64)
	for _, s := range d.Streams {
		switch s.CodecType {
		case "video":
			fps := parseRate(s.AvgFrameRate)
			if fps == 0 {
				fps = parseRate(s.RFrameRate)
			}
			br, _ := strconv.ParseInt(s.BitRate, 10, 64)
			r.Video = append(r.Video, VideoStream{Index: s.Index, Codec: s.CodecName, Width: s.Width, Height: s.Height, FPS: fps, Bitrate: br})
		case "audio":
			sr, _ := strconv.Atoi(s.SampleRate)
			r.Audio = append(r.Audio, AudioStream{Index: s.Index, Codec: s.CodecName, SampleRate: sr, Channels: s.Channels})
		}
	}
	if len(r.Video) == 0 && len(r.Audio) == 0 {
		return ProbeResult{}, errors.New("ffprobe returned no media streams")
	}
	return r, nil
}
func parseRate(raw string) float64 {
	a, b, ok := strings.Cut(raw, "/")
	if !ok {
		v, _ := strconv.ParseFloat(raw, 64)
		return v
	}
	n, e1 := strconv.ParseFloat(a, 64)
	d, e2 := strconv.ParseFloat(b, 64)
	if e1 != nil || e2 != nil || d == 0 {
		return 0
	}
	return n / d
}
