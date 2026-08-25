package config

import (
	"encoding/json"
	"errors"
)

func Render(existing map[string]any, desired Managed, previous, next Ownership) ([]byte, error) {
	if existing == nil {
		existing = map[string]any{}
	}
	// Deep-copy through JSON so callers never observe mutation.
	raw, err := json.Marshal(existing)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err = json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}

	if next.ManageMQTT && desired.MQTT != nil {
		root["mqtt"] = toAny(*desired.MQTT)
	}
	if next.ManageGlobalFFmpeg && desired.FFmpeg != nil {
		root["ffmpeg"] = toAny(*desired.FFmpeg)
	}
	if next.ManageRecord && desired.Record != nil {
		root["record"] = toAny(*desired.Record)
	}
	if next.ManageSnapshots && desired.Snapshots != nil {
		root["snapshots"] = toAny(*desired.Snapshots)
	}

	camerasMap := mapFrom(root["cameras"])
	for _, name := range previous.CameraNames {
		if !contains(next.CameraNames, name) {
			delete(camerasMap, name)
		}
	}
	for name, c := range desired.Cameras {
		if contains(next.CameraNames, name) {
			camerasMap[name] = toAny(c)
		}
	}
	root["cameras"] = camerasMap

	g := mapFrom(root["go2rtc"])
	streams := mapFrom(g["streams"])
	for _, name := range previous.StreamNames {
		if !contains(next.StreamNames, name) {
			delete(streams, name)
		}
	}
	for name, sources := range desired.Go2RTC.Streams {
		if contains(next.StreamNames, name) {
			streams[name] = sources
		}
	}
	g["streams"] = streams
	if previous.ManageGo2RTCWebRTC && !next.ManageGo2RTCWebRTC {
		delete(g, "webrtc")
	}
	if next.ManageGo2RTCWebRTC && desired.Go2RTC.WebRTC != nil {
		g["webrtc"] = toAny(*desired.Go2RTC.WebRTC)
	}
	root["go2rtc"] = g

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	if !json.Valid(out) {
		return nil, errors.New("generated Frigate JSON is invalid")
	}
	return append(out, '\n'), nil
}
func mapFrom(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
func toAny(v any) any { b, _ := json.Marshal(v); var out any; _ = json.Unmarshal(b, &out); return out }
func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
