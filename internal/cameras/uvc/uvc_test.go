package uvc

import "testing"

func TestParseModes(t *testing.T) {
	raw := `[0]: 'MJPG' (Motion-JPEG, compressed)
    Size: Discrete 1920x1080
        Interval: Discrete 0.033s (30.000 fps)
        Interval: Discrete 0.067s (15.000 fps)
[1]: 'YUYV' (YUYV 4:2:2)
    Size: Discrete 640x480
        Interval: Discrete 0.033s (30.000 fps)`
	m := parseModes(raw)
	if len(m) != 3 {
		t.Fatalf("modes=%+v", m)
	}
	if m[0].PixelFormat != "MJPG" || m[0].Width != 1920 || m[0].FPS != 30 {
		t.Fatalf("first=%+v", m[0])
	}
}

func TestParseDShowDevices(t *testing.T) {
	raw := `[in#0 @ 000002888cce5240] "Microsoft LifeCam" (video)
[in#0 @ 000002888cce5240]   Alternative name "@device_pnp_\\?\usb#vid_045e&pid_074a&mi_00#6&6c478b7&0&0000#{65e8773d-8f56-11d0-a3b9-00a0c9223196}\global"
[in#0 @ 000002888cce5240] "Integrated Camera" (video)
[in#0 @ 000002888cce5240] Could not enumerate audio only devices (or none found).`
	devs := parseDShowDevices(raw)
	if len(devs) != 2 {
		t.Fatalf("expected 2 devices, got %+v", devs)
	}
	if devs[0].Name != "Microsoft LifeCam" || devs[0].Path != "video=Microsoft LifeCam" {
		t.Fatalf("dev0=%+v", devs[0])
	}
	if devs[1].Name != "Integrated Camera" {
		t.Fatalf("dev1=%+v", devs[1])
	}
}

func TestParseDShowModes(t *testing.T) {
	raw := `[in#0 @ 0000025699b74a80] DirectShow video device options (from video devices)
[in#0 @ 0000025699b74a80]  Pin "Capture" (alternative pin name "Capture")
[in#0 @ 0000025699b74a80]   pixel_format=yuyv422  min s=640x480 fps=1 max s=640x480 fps=30
[in#0 @ 0000025699b74a80]   pixel_format=mjpeg    min s=1280x720 fps=1 max s=1280x720 fps=30
[in#0 @ 0000025699b74a80]   pixel_format=yuyv422  min s=320x240 fps=1 max s=320x240 fps=30`
	modes := parseDShowModes(raw)
	if len(modes) != 3 {
		t.Fatalf("expected 3 modes, got %+v", modes)
	}
	if modes[0].PixelFormat != "yuyv422" || modes[0].Width != 640 || modes[0].Height != 480 || modes[0].FPS != 30 {
		t.Fatalf("mode 0 = %+v", modes[0])
	}
	if modes[1].PixelFormat != "mjpeg" || modes[1].Width != 1280 || modes[1].Height != 720 || modes[1].FPS != 30 {
		t.Fatalf("mode 1 = %+v", modes[1])
	}
}
