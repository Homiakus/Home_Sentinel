package hardware

import (
	"context"
	"time"
)

func Collect(ctx context.Context, r Runner) Profile {
	return Profile{CollectedAt: time.Now().UTC(), OS: ProbeOS(), CPU: ProbeCPU(), Memory: ProbeMemory(), Video: ProbeVideo(ctx, r), Storage: ProbeStorage(), Network: ProbeNetwork()}
}

type Recommendation struct {
	VideoDecoder  string   `json:"video_decoder"`
	Detector      string   `json:"detector"`
	AIProfile     string   `json:"ai_profile"`
	AIMaxParallel int      `json:"ai_max_parallel"`
	Reasons       []string `json:"reasons"`
	Warnings      []string `json:"warnings"`
}

func Recommend(p Profile) Recommendation {
	r := Recommendation{VideoDecoder: "cpu", Detector: "cpu", AIProfile: "OFF", AIMaxParallel: 1}
	if p.Video.VAAPI.Available {
		r.VideoDecoder = "vaapi"
		r.Reasons = append(r.Reasons, "VAAPI decode capability detected")
	}
	var maxVRAM int
	for _, g := range p.Video.NVIDIA {
		if g.VRAMMiB > maxVRAM {
			maxVRAM = g.VRAMMiB
		}
	}
	if maxVRAM > 0 {
		r.VideoDecoder = "nvidia"
		r.Detector = "nvidia"
		r.Reasons = append(r.Reasons, "NVIDIA GPU visible to host probe")
	}
	gib := p.Memory.Total / (1 << 30)
	switch {
	case maxVRAM >= 12000 && gib >= 24:
		r.AIProfile = "HIGH"
		r.AIMaxParallel = 2
	case maxVRAM >= 6000 && gib >= 16:
		r.AIProfile = "BALANCED"
	case p.Video.VAAPI.Available && gib >= 16:
		r.AIProfile = "LIGHT"
	case gib >= 12:
		r.AIProfile = "LIGHT"
		r.Warnings = append(r.Warnings, "AI will likely run on CPU unless another accelerator is configured")
	default:
		r.Warnings = append(r.Warnings, "Insufficient detected memory/acceleration for default local VLM profile")
	}
	if p.OS.CgroupMemoryLimit > 0 && p.OS.CgroupMemoryLimit < p.Memory.Total {
		r.Warnings = append(r.Warnings, "Container cgroup memory limit is below host memory")
	}
	return r
}
