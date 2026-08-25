package frigate

import (
	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/hardware"
	"testing"
)

func TestMapCameraMainAndDetectRoles(t *testing.T) {
	cam := cameras.Camera{ID: "cam_x", Name: "X", Type: cameras.TypeRTSP, Streams: []cameras.Stream{{ID: "m", Role: cameras.RoleMain, Endpoint: cameras.Endpoint{URL: "rtsp://10.0.0.2/main"}, Width: 2560, Height: 1440, FPS: 20}, {ID: "d", Role: cameras.RoleDetect, Endpoint: cameras.Endpoint{URL: "rtsp://10.0.0.2/sub"}, Width: 640, Height: 360, FPS: 10}}}
	m, err := MapCamera(cam, hardware.Recommendation{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Config.FFmpeg.Inputs) != 2 {
		t.Fatal("expected two inputs")
	}
	roles := map[string][]string{}
	for _, in := range m.Config.FFmpeg.Inputs {
		roles[in.Path] = in.Roles
	}
	if got := m.Config.Detect.FPS; got != 5 {
		t.Fatalf("detect fps=%v", got)
	}
	var record, detect bool
	for _, in := range m.Config.FFmpeg.Inputs {
		for _, r := range in.Roles {
			if r == "record" && containsString(in.Path, "cam_x") && !containsString(in.Path, "_sub") {
				record = true
			}
			if r == "detect" && containsString(in.Path, "_sub") {
				detect = true
			}
		}
	}
	if !record || !detect {
		t.Fatalf("inputs=%+v", m.Config.FFmpeg.Inputs)
	}
}
func containsString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
func TestHardwarePreset(t *testing.T) {
	if HardwareFFmpeg(hardware.Recommendation{VideoDecoder: "nvidia"}).HWAccelArgs != "preset-nvidia" {
		t.Fatal("bad preset")
	}
}
