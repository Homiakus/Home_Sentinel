package httpserver

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/health"
	setupsvc "github.com/Homiakus/Home_Sentinel/internal/setup"
)

func (s *Server) setupWizard(w http.ResponseWriter, r *http.Request) {
	snap := setupsvc.WizardSnapshot{Admin: s.app.Users != nil, Storage: len(s.app.Hardware.Storage) > 0, Network: len(s.app.Config.Network.CameraCIDRs) > 0, MQTTEnabled: s.app.Config.MQTT.Enabled, HAEnabled: s.app.Config.HomeAssistant.Enabled, FrigateEnabled: s.app.Config.Frigate.Enabled, AIEnabled: s.app.Config.AI.Enabled, TelegramEnabled: s.app.Config.Telegram.Enabled, BackupEnabled: s.app.Config.Backup.Enabled}
	if s.app.Users != nil {
		if n, e := s.app.Users.Count(r.Context()); e == nil {
			snap.Admin = n > 0
		}
	}
	isHealthy := func(name string) bool { c, ok := s.app.Health.Get(name); return ok && c.Status == health.Healthy }
	snap.MQTTHealthy = isHealthy("mqtt")
	snap.HAHealthy = isHealthy("home_assistant")
	snap.FrigateHealthy = isHealthy("frigate")
	snap.AIHealthy = isHealthy("ai")
	snap.TelegramHealthy = isHealthy("telegram")
	snap.BackupHealthy = isHealthy("backup")
	if s.app.Cameras != nil {
		if x, e := s.app.Cameras.List(r.Context(), 500); e == nil {
			snap.CameraCount = len(x)
		}
	}
	if s.app.Intercom != nil {
		if x, e := s.app.Intercom.List(r.Context(), 500); e == nil {
			snap.IntercomCount = len(x)
		}
	}
	state := setupsvc.EvaluateWizard(snap)
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) setupVerify(w http.ResponseWriter, r *http.Request) {
	checks := []setupsvc.VerificationCheck{}
	add := func(id, name string, ok bool, detail string, d time.Duration) {
		status := setupsvc.VerifyPass
		if !ok {
			status = setupsvc.VerifyFail
		}
		checks = append(checks, setupsvc.VerificationCheck{ID: id, Name: name, Status: status, Detail: detail, Duration: d.Milliseconds()})
	}

	start := time.Now()
	if _, err := database.SchemaVersion(r.Context(), s.app.DB); err != nil {
		add("database", "Database", false, "schema unavailable", time.Since(start))
	} else {
		add("database", "Database", true, "schema readable", time.Since(start))
	}

	cams, err := s.app.Cameras.List(r.Context(), 100)
	if err != nil {
		add("cameras", "Cameras", false, "camera inventory unavailable", 0)
	} else if len(cams) == 0 {
		add("cameras", "Cameras", false, "at least one camera is required", 0)
	} else {
		for _, cam := range cams {
			stream, ok := cam.StreamByRole(cameras.RoleMain)
			if !ok && len(cam.Streams) > 0 {
				stream = cam.Streams[0]
				ok = true
			}
			if !ok || stream.Endpoint.URL == "" {
				add("camera:"+cam.ID, cam.Name, false, "no probeable stream", 0)
				continue
			}
			t0 := time.Now()
			p, probeErr := s.app.Cameras.ProbeRTSP(r.Context(), stream.Endpoint)
			detail := "stream decoded"
			if probeErr == nil && len(p.Probe.Media.Video) > 0 {
				v := p.Probe.Media.Video[0]
				detail = fmt.Sprintf("%s %dx%d %.1ffps", v.Codec, v.Width, v.Height, v.FPS)
			}
			if probeErr != nil {
				detail = "media probe failed"
			}
			add("camera:"+cam.ID, cam.Name, probeErr == nil, detail, time.Since(t0))
		}
	}

	if s.app.Config.Frigate.Enabled {
		t0 := time.Now()
		ok := false
		detail := "Frigate unavailable"
		if s.app.Frigate != nil {
			if _, _, e := s.app.Frigate.Capabilities(r.Context()); e == nil {
				ok = true
				detail = "Frigate API verified"
			}
		}
		add("frigate", "Frigate", ok, detail, time.Since(t0))
	}
	for _, dep := range []struct {
		name, title string
		enabled     bool
	}{
		{"mqtt", "MQTT", s.app.Config.MQTT.Enabled},
		{"home_assistant", "Home Assistant", s.app.Config.HomeAssistant.Enabled},
		{"ai", "Local AI", s.app.Config.AI.Enabled},
		{"telegram", "Telegram", s.app.Config.Telegram.Enabled},
		{"backup", "Backup", s.app.Config.Backup.Enabled},
	} {
		if !dep.enabled {
			checks = append(checks, setupsvc.VerificationCheck{ID: dep.name, Name: dep.title, Status: setupsvc.VerifySkip, Detail: "disabled"})
			continue
		}
		c, ok := s.app.Health.Get(dep.name)
		healthy := ok && c.Status == health.Healthy
		detail := "health not verified"
		if ok {
			detail = string(c.Status)
			if c.Cause != "" {
				detail += ": " + c.Cause
			}
		}
		add(dep.name, dep.title, healthy, detail, 0)
	}
	writeJSON(w, http.StatusOK, setupsvc.NewVerification(checks...))
}
