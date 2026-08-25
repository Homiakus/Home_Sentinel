package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	resticint "github.com/Homiakus/Home_Sentinel/internal/backup/restic"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type backupRestoreTestRequest struct {
	Snapshot string `json:"snapshot"`
}

type backupRetentionRequest struct {
	KeepHourly  int `json:"keep_hourly"`
	KeepDaily   int `json:"keep_daily"`
	KeepWeekly  int `json:"keep_weekly"`
	KeepMonthly int `json:"keep_monthly"`
}

func (s *Server) backupStatus(w http.ResponseWriter, r *http.Request) {
	if s.app.Backup == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	jobs, err := s.app.Backup.Jobs.List(r.Context(), 20)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "BACKUP_STATUS_FAILED", "Unable to read backup history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":    true,
		"repository": s.app.Config.Backup.Repository,
		"interval":   s.app.Config.Backup.Interval.String(),
		"retention": map[string]int{
			"keep_hourly":  s.app.Config.Backup.KeepHourly,
			"keep_daily":   s.app.Config.Backup.KeepDaily,
			"keep_weekly":  s.app.Config.Backup.KeepWeekly,
			"keep_monthly": s.app.Config.Backup.KeepMonthly,
		},
		"jobs": jobs,
	})
}

func (s *Server) backupInit(w http.ResponseWriter, r *http.Request) {
	if !s.requireBackup(w, r) {
		return
	}
	if err := s.app.Backup.Init(r.Context()); err != nil {
		s.auditBackup(r, "backup.repository.init", "failed", map[string]any{"error": err.Error()})
		writeProblem(w, r, http.StatusBadGateway, "BACKUP_INIT_FAILED", "Unable to initialize backup repository")
		return
	}
	s.auditBackup(r, "backup.repository.init", "success", nil)
	writeJSON(w, http.StatusOK, map[string]any{"status": "initialized"})
}

func (s *Server) backupRun(w http.ResponseWriter, r *http.Request) {
	if !s.requireBackup(w, r) {
		return
	}
	job, err := s.app.Backup.RunCritical(r.Context())
	if err != nil {
		s.auditBackup(r, "backup.run", "failed", map[string]any{"job_id": job.ID, "error": err.Error()})
		writeJSON(w, http.StatusBadGateway, map[string]any{"job": job, "error": "backup failed"})
		return
	}
	s.auditBackup(r, "backup.run", "success", map[string]any{"job_id": job.ID, "snapshot_id": job.SnapshotID})
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) backupCheck(w http.ResponseWriter, r *http.Request) {
	if !s.requireBackup(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Hour)
	defer cancel()
	if err := s.app.Backup.Check(ctx); err != nil {
		s.auditBackup(r, "backup.check", "failed", map[string]any{"error": err.Error()})
		writeProblem(w, r, http.StatusBadGateway, "BACKUP_CHECK_FAILED", "Backup repository integrity check failed")
		return
	}
	s.auditBackup(r, "backup.check", "success", nil)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) backupRestoreTest(w http.ResponseWriter, r *http.Request) {
	if !s.requireBackup(w, r) {
		return
	}
	var in backupRestoreTestRequest
	if err := decodeBody(w, r, &in); err != nil || in.Snapshot == "" {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_RESTORE_TEST", "snapshot is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Hour)
	defer cancel()
	ok, err := s.app.Backup.RestoreTest(ctx, in.Snapshot)
	if err != nil || !ok {
		details := map[string]any{"snapshot": in.Snapshot}
		if err != nil {
			details["error"] = err.Error()
		}
		s.auditBackup(r, "backup.restore_test", "failed", details)
		writeProblem(w, r, http.StatusBadGateway, "RESTORE_TEST_FAILED", "Backup restore verification failed")
		return
	}
	s.auditBackup(r, "backup.restore_test", "success", map[string]any{"snapshot": in.Snapshot})
	writeJSON(w, http.StatusOK, map[string]any{"snapshot": in.Snapshot, "verified": true})
}

func (s *Server) backupRetentionPreview(w http.ResponseWriter, r *http.Request) {
	if !s.requireBackup(w, r) {
		return
	}
	ret, ok := decodeRetention(w, r)
	if !ok {
		return
	}
	if err := s.app.Backup.PreviewRetention(r.Context(), ret); err != nil {
		writeProblem(w, r, http.StatusBadGateway, "BACKUP_RETENTION_PREVIEW_FAILED", "Unable to preview retention policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "preview_ok"})
}

func (s *Server) backupRetentionApply(w http.ResponseWriter, r *http.Request) {
	if !s.requireBackup(w, r) {
		return
	}
	ret, ok := decodeRetention(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Hour)
	defer cancel()
	if err := s.app.Backup.ApplyRetention(ctx, ret); err != nil {
		s.auditBackup(r, "backup.retention.apply", "failed", map[string]any{"error": err.Error()})
		writeProblem(w, r, http.StatusBadGateway, "BACKUP_RETENTION_FAILED", "Unable to apply backup retention")
		return
	}
	s.auditBackup(r, "backup.retention.apply", "success", map[string]any{"retention": ret})
	writeJSON(w, http.StatusOK, map[string]any{"status": "applied_and_checked"})
}

func decodeRetention(w http.ResponseWriter, r *http.Request) (resticint.Retention, bool) {
	var in backupRetentionRequest
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "INVALID_RETENTION", "Invalid retention request")
		return resticint.Retention{}, false
	}
	if in.KeepHourly < 0 || in.KeepDaily < 0 || in.KeepWeekly < 0 || in.KeepMonthly < 0 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "INVALID_RETENTION", "Retention values cannot be negative")
		return resticint.Retention{}, false
	}
	return resticint.Retention{KeepHourly: in.KeepHourly, KeepDaily: in.KeepDaily, KeepWeekly: in.KeepWeekly, KeepMonthly: in.KeepMonthly}, true
}

func (s *Server) requireBackup(w http.ResponseWriter, r *http.Request) bool {
	if s.app.Backup == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "BACKUP_DISABLED", "Backup service is not enabled")
		return false
	}
	return true
}

func (s *Server) auditBackup(r *http.Request, action, result string, details any) {
	if s.app.Audit == nil {
		return
	}
	p, _ := principalFrom(r.Context())
	var body []byte
	if details != nil {
		body, _ = json.Marshal(details)
	}
	_, _ = s.app.Audit.Append(r.Context(), repository.AuditEntry{Actor: p.User.ID, Source: "web", Action: action, Target: "backup", Result: result, RequestID: requestIDFrom(r.Context()), Details: body})
}
