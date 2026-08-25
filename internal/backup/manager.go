package backup

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	resticint "github.com/Homiakus/Home_Sentinel/internal/backup/restic"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type Restic interface {
	Init(context.Context) error
	Backup(context.Context, []string, []string, []string) (resticint.BackupResult, error)
	Check(context.Context) error
	Restore(context.Context, string, string) error
	Forget(context.Context, resticint.Retention, bool) error
	Prune(context.Context) error
}
type JobRecord struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at,omitempty"`
	Status          string    `json:"status"`
	SnapshotID      string    `json:"snapshot_id,omitempty"`
	Error           string    `json:"error,omitempty"`
	RestoreVerified bool      `json:"restore_verified"`
}
type Manager struct {
	DB                  *sql.DB
	Restic              Restic
	Jobs                *repository.Store[JobRecord]
	ConfigFiles         []string
	SecretRoot          string
	ExcludedSecretNames []string
	OnResult            func(operation string, err error)
	mu                  sync.Mutex
}

func (m *Manager) RunCritical(ctx context.Context) (JobRecord, error) {
	if m == nil || m.DB == nil || m.Restic == nil {
		return JobRecord{}, errors.New("backup manager unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	job := JobRecord{ID: "backup_" + time.Now().UTC().Format("20060102T150405.000000000"), Type: "critical", StartedAt: time.Now().UTC(), Status: "running"}
	if m.Jobs != nil {
		_, _ = m.Jobs.Put(ctx, job.ID, job)
	}
	bundle, err := PrepareCriticalBundle(ctx, m.DB, m.ConfigFiles, m.SecretRoot, m.ExcludedSecretNames)
	if err == nil {
		defer bundle.Cleanup()
		set := DefaultCriticalSet(bundle.Root)
		var result resticint.BackupResult
		result, err = m.Restic.Backup(ctx, set.Paths, set.Tags, set.Excludes)
		job.SnapshotID = result.SnapshotID
	}
	job.FinishedAt = time.Now().UTC()
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
	} else {
		job.Status = "succeeded"
	}
	if m.Jobs != nil {
		_, _ = m.Jobs.Put(context.Background(), job.ID, job)
	}
	m.result("backup", err)
	return job, err
}
func (m *Manager) Check(ctx context.Context) error {
	if m == nil || m.Restic == nil {
		return errors.New("backup manager unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.Restic.Check(ctx)
	m.result("check", err)
	return err
}
func (m *Manager) RestoreTest(ctx context.Context, snapshot string) (ok bool, err error) {
	if m == nil || m.Restic == nil {
		return false, errors.New("backup manager unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	defer func() { m.result("restore_test", err) }()
	root, err := os.MkdirTemp("", "sentinel-restore-test-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(root)
	if err := m.Restic.Restore(ctx, snapshot, root); err != nil {
		return false, err
	}
	bundleRoot, err := findBundleRoot(root)
	if err != nil {
		return false, err
	}
	if err := VerifyCriticalBundle(bundleRoot); err != nil {
		return false, err
	}
	dbPath := filepath.Join(bundleRoot, "state", "sentinel.db")
	testDB, err := database.Open(ctx, database.Options{Path: dbPath, BusyTimeout: 2 * time.Second})
	if err != nil {
		return false, err
	}
	defer testDB.Close()
	var integrity string
	if err := testDB.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return false, err
	}
	if strings.ToLower(integrity) != "ok" {
		return false, errors.New("restored SQLite integrity check failed")
	}
	return true, nil
}
func (m *Manager) Init(ctx context.Context) error {
	if m == nil || m.Restic == nil {
		return errors.New("backup manager unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.Restic.Init(ctx)
	m.result("init", err)
	return err
}
func (m *Manager) PreviewRetention(ctx context.Context, r resticint.Retention) error {
	if m == nil || m.Restic == nil {
		return errors.New("backup manager unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.Restic.Forget(ctx, r, true)
	m.result("retention_preview", err)
	return err
}
func (m *Manager) ApplyRetention(ctx context.Context, r resticint.Retention) error {
	if m == nil || m.Restic == nil {
		return errors.New("backup manager unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.Restic.Forget(ctx, r, false); err != nil {
		m.result("retention_apply", err)
		return err
	}
	if err := m.Restic.Prune(ctx); err != nil {
		m.result("retention_apply", err)
		return err
	}
	err := m.Restic.Check(ctx)
	m.result("retention_apply", err)
	return err
}

func findBundleRoot(root string) (string, error) {
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() == "manifest.json" {
			candidate := filepath.Dir(path)
			if _, err := os.Stat(filepath.Join(candidate, "state", "sentinel.db")); err == nil {
				found = candidate
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", errors.New("restored critical bundle not found")
	}
	return found, nil
}

func (m *Manager) result(operation string, err error) {
	if m != nil && m.OnResult != nil {
		m.OnResult(operation, err)
	}
}
