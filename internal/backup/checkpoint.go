package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func ExportCriticalToDirectory(ctx context.Context, db *sql.DB, configFiles []string, secretRoot string, target string) error {
	if target == "" {
		return errors.New("checkpoint target required")
	}
	if _, e := os.Stat(target); e == nil {
		return errors.New("checkpoint target already exists")
	}
	b, e := PrepareCriticalBundle(ctx, db, configFiles, secretRoot, nil)
	if e != nil {
		return e
	}
	defer b.Cleanup()
	tmp := target + ".tmp"
	_ = os.RemoveAll(tmp)
	if e = copyTree(b.Root, tmp, nil); e != nil {
		_ = os.RemoveAll(tmp)
		return e
	}
	if e = VerifyCriticalBundle(tmp); e != nil {
		_ = os.RemoveAll(tmp)
		return e
	}
	if e = os.Rename(tmp, target); e != nil {
		_ = os.RemoveAll(tmp)
		return e
	}
	return nil
}
func RestoreCriticalFromDirectory(bundleRoot, dbPath, secretRoot string) error {
	if bundleRoot == "" || dbPath == "" {
		return errors.New("checkpoint source and database path required")
	}
	if e := VerifyCriticalBundle(bundleRoot); e != nil {
		return fmt.Errorf("verify checkpoint: %w", e)
	}
	src := filepath.Join(bundleRoot, "state", "sentinel.db")
	if _, e := os.Stat(src); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(dbPath), 0700); e != nil {
		return e
	}
	tmp := dbPath + ".restore.tmp"
	if e := copyFile(src, tmp, 0600); e != nil {
		return e
	}
	old := dbPath + ".pre-restore." + time.Now().UTC().Format("20060102T150405")
	if _, e := os.Stat(dbPath); e == nil {
		if e = os.Rename(dbPath, old); e != nil {
			return e
		}
	}
	if e := os.Rename(tmp, dbPath); e != nil {
		return e
	}
	srcSecrets := filepath.Join(bundleRoot, "secrets")
	if secretRoot != "" {
		if st, e := os.Stat(srcSecrets); e == nil && st.IsDir() {
			tmpSecrets := secretRoot + ".restore.tmp"
			_ = os.RemoveAll(tmpSecrets)
			if e = copyTree(srcSecrets, tmpSecrets, nil); e != nil {
				return e
			}
			oldSecrets := secretRoot + ".pre-restore"
			_ = os.RemoveAll(oldSecrets)
			if _, e = os.Stat(secretRoot); e == nil {
				if e = os.Rename(secretRoot, oldSecrets); e != nil {
					return e
				}
			}
			if e = os.Rename(tmpSecrets, secretRoot); e != nil {
				return e
			}
		}
	}
	return nil
}
