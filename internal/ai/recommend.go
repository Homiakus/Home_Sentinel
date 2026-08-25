package ai

import "github.com/Homiakus/Home_Sentinel/internal/hardware"

type RuntimeProfile struct {
	Level       string   `json:"level"`
	MaxParallel int      `json:"max_parallel"`
	MaxFrames   int      `json:"max_frames"`
	Reasons     []string `json:"reasons"`
	Warnings    []string `json:"warnings,omitempty"`
}

func Recommend(p hardware.Profile) RuntimeProfile {
	h := hardware.Recommend(p)
	r := RuntimeProfile{Level: h.AIProfile, MaxParallel: h.AIMaxParallel, MaxFrames: 2, Reasons: append([]string(nil), h.Reasons...), Warnings: append([]string(nil), h.Warnings...)}
	switch r.Level {
	case "HIGH":
		r.MaxFrames = 6
	case "BALANCED":
		r.MaxFrames = 4
	case "LIGHT":
		r.MaxFrames = 2
	default:
		r.MaxParallel = 0
		r.MaxFrames = 0
	}
	return r
}
