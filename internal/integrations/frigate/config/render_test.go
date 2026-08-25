package config

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRenderPreservesManualAndRemovesOldManaged(t *testing.T) {
	existing := map[string]any{"manual": map[string]any{"x": float64(1)}, "cameras": map[string]any{"manual_cam": map[string]any{"enabled": true}, "cam_old": map[string]any{}}, "go2rtc": map[string]any{"streams": map[string]any{"manual_stream": []any{"x"}, "cam_old": []any{"old"}}}}
	d := Managed{Go2RTC: Go2RTC{Streams: map[string][]string{"cam_new": {"rtsp://127.0.0.1/x"}}}, Cameras: map[string]Camera{"cam_new": {FFmpeg: CameraFFmpeg{Inputs: []Input{{Path: "rtsp://127.0.0.1:8554/cam_new", Roles: []string{"detect", "record"}}}}}}}
	prev := Ownership{CameraNames: []string{"cam_old"}, StreamNames: []string{"cam_old"}}
	next := Ownership{CameraNames: []string{"cam_new"}, StreamNames: []string{"cam_new"}}
	a, err := Render(existing, d, prev, next)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(existing, d, prev, next)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("render not deterministic")
	}
	var got map[string]any
	_ = json.Unmarshal(a, &got)
	cams := got["cameras"].(map[string]any)
	if cams["manual_cam"] == nil || cams["cam_new"] == nil || cams["cam_old"] != nil {
		t.Fatalf("cameras=%v", cams)
	}
	if got["manual"] == nil {
		t.Fatal("manual section lost")
	}
}

func TestRenderManagedWebRTCCandidatesPreservesOtherGo2RTCSettings(t *testing.T) {
	existing := map[string]any{"go2rtc": map[string]any{"api": map[string]any{"listen": ":1984"}, "webrtc": map[string]any{"candidates": []any{"old:8555"}}, "streams": map[string]any{}}}
	d := Managed{Go2RTC: Go2RTC{Streams: map[string][]string{}, WebRTC: &WebRTC{Candidates: []string{"192.168.1.10:8555"}}}, Cameras: map[string]Camera{}}
	out, err := Render(existing, d, Ownership{ManageGo2RTCWebRTC: true}, Ownership{ManageGo2RTCWebRTC: true})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	g := got["go2rtc"].(map[string]any)
	if g["api"] == nil {
		t.Fatal("manual go2rtc api config was lost")
	}
	w := g["webrtc"].(map[string]any)
	candidates := w["candidates"].([]any)
	if len(candidates) != 1 || candidates[0] != "192.168.1.10:8555" {
		t.Fatalf("candidates=%v", candidates)
	}
}
