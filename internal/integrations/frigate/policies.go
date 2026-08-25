package frigate

import (
	"errors"
	"sort"

	fgconfig "github.com/Homiakus/Home_Sentinel/internal/integrations/frigate/config"
)

type CameraPolicy struct {
	RecordingEnabled bool
	ContinuousDays   float64
	MotionDays       float64
	AlertDays        float64
	DetectionDays    float64
	SnapshotsEnabled bool
	SnapshotDays     float64
	DetectionEnabled bool
	TrackObjects     []string
	Zones            []ZonePolicy
}
type ZonePolicy struct {
	Name        string
	Coordinates string
	Objects     []string
}

func DefaultCameraPolicy() CameraPolicy {
	return CameraPolicy{RecordingEnabled: true, ContinuousDays: 3, MotionDays: 7, AlertDays: 30, DetectionDays: 30, SnapshotsEnabled: true, SnapshotDays: 30, DetectionEnabled: true, TrackObjects: []string{"person", "car"}}
}

func ApplyPolicy(cam *fgconfig.Camera, p CameraPolicy) error {
	if cam == nil {
		return errors.New("nil Frigate camera config")
	}
	if p.ContinuousDays < 0 || p.MotionDays < 0 || p.AlertDays < 0 || p.DetectionDays < 0 || p.SnapshotDays < 0 {
		return errors.New("retention days cannot be negative")
	}
	cam.Detect.Enabled = fgconfig.Bool(p.DetectionEnabled)
	cam.Record = &fgconfig.Record{Enabled: fgconfig.Bool(p.RecordingEnabled), Continuous: &fgconfig.RetainDays{Days: p.ContinuousDays}, Motion: &fgconfig.RetainDays{Days: p.MotionDays}, Alerts: &fgconfig.RetainBlock{Retain: fgconfig.Retain{Days: p.AlertDays, Mode: "motion"}}, Detections: &fgconfig.RetainBlock{Retain: fgconfig.Retain{Days: p.DetectionDays, Mode: "motion"}}}
	cam.Snapshots = &fgconfig.Snapshots{Enabled: fgconfig.Bool(p.SnapshotsEnabled), Retain: &fgconfig.SnapshotRetain{Default: p.SnapshotDays}}
	if len(p.TrackObjects) > 0 {
		objs := append([]string(nil), p.TrackObjects...)
		sort.Strings(objs)
		cam.Objects = &fgconfig.Objects{Track: objs}
	}
	if len(p.Zones) > 0 {
		cam.Zones = map[string]fgconfig.Zone{}
		for _, z := range p.Zones {
			if z.Name == "" || z.Coordinates == "" {
				return errors.New("zone name and coordinates required")
			}
			objs := append([]string(nil), z.Objects...)
			sort.Strings(objs)
			cam.Zones[z.Name] = fgconfig.Zone{Coordinates: z.Coordinates, Objects: objs}
		}
	}
	return nil
}
