package frigate

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/hardware"
	fgconfig "github.com/Homiakus/Home_Sentinel/internal/integrations/frigate/config"
	"github.com/Homiakus/Home_Sentinel/internal/integrations/go2rtc"
)

type CameraMapping struct {
	Name   string
	Config fgconfig.Camera
	Go2RTC go2rtc.Generated
}

func MapCamera(cam cameras.Camera, hw hardware.Recommendation, resolver go2rtc.SecretResolver) (CameraMapping, error) {
	if err := cam.Validate(); err != nil {
		return CameraMapping{}, err
	}
	generated, err := go2rtc.Generate(cam, resolver)
	if err != nil {
		return CameraMapping{}, err
	}
	main, hasMain := cam.StreamByRole(cameras.RoleMain)
	detect, hasDetect := cam.StreamByRole(cameras.RoleDetect)
	if !hasMain && hasDetect {
		main = detect
		hasMain = true
	}
	if !hasMain {
		return CameraMapping{}, errors.New("camera has no main/detect stream")
	}
	name := go2rtc.CanonicalStreamName(cam.ID, cameras.RoleMain)
	inputs := []fgconfig.Input{}
	if hasDetect && detect.ID != main.ID {
		inputs = append(inputs, fgconfig.Input{Path: fmt.Sprintf("rtsp://127.0.0.1:8554/%s", go2rtc.CanonicalStreamName(cam.ID, cameras.RoleMain)), InputArgs: "preset-rtsp-restream", Roles: []string{"record"}})
		inputs = append(inputs, fgconfig.Input{Path: fmt.Sprintf("rtsp://127.0.0.1:8554/%s", go2rtc.CanonicalStreamName(cam.ID, cameras.RoleDetect)), InputArgs: "preset-rtsp-restream", Roles: []string{"detect"}})
	} else {
		inputs = append(inputs, fgconfig.Input{Path: fmt.Sprintf("rtsp://127.0.0.1:8554/%s", go2rtc.CanonicalStreamName(cam.ID, cameras.RoleMain)), InputArgs: "preset-rtsp-restream", Roles: []string{"detect", "record"}})
		detect = main
	}
	sort.SliceStable(inputs, func(i, j int) bool { return inputs[i].Path < inputs[j].Path })
	fps := detect.FPS
	if fps <= 0 || fps > 5 {
		fps = 5
	}
	fc := fgconfig.Camera{Enabled: fgconfig.Bool(true), FFmpeg: fgconfig.CameraFFmpeg{Inputs: inputs}, Detect: fgconfig.Detect{Enabled: fgconfig.Bool(true), Width: detect.Width, Height: detect.Height, FPS: fps}}
	// Preserve audio in recordings when the source contains it. Audio detection is a separate policy.
	if cam.Capabilities.Audio {
		fc.FFmpeg.OutputArgs = &fgconfig.OutputArgs{Record: "preset-record-generic-audio-copy"}
	}
	_ = hw // global hwaccel is mapped independently so one preset remains consistent across cameras.
	return CameraMapping{Name: name, Config: fc, Go2RTC: generated}, nil
}

func HardwareFFmpeg(r hardware.Recommendation) *fgconfig.GlobalFFmpeg {
	switch r.VideoDecoder {
	case "vaapi":
		return &fgconfig.GlobalFFmpeg{HWAccelArgs: "preset-vaapi"}
	case "nvidia":
		return &fgconfig.GlobalFFmpeg{HWAccelArgs: "preset-nvidia"}
	default:
		return nil
	}
}
