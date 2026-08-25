package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type CriticalManifest struct {
	Schema    int            `json:"schema"`
	CreatedAt time.Time      `json:"created_at"`
	Files     []ManifestFile `json:"files"`
}
type CriticalBundle struct {
	Root    string
	Cleanup func()
}

func PrepareCriticalBundle(ctx context.Context, db *sql.DB, configFiles []string, secretRoot string, excludedSecretNames []string) (CriticalBundle, error) {
	if db == nil {
		return CriticalBundle{}, errors.New("database required")
	}
	root, err := os.MkdirTemp("", "home-sentinel-critical-")
	if err != nil {
		return CriticalBundle{}, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	fail := func(e error) (CriticalBundle, error) { cleanup(); return CriticalBundle{}, e }
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fail(err)
	}
	snapshot := filepath.Join(stateDir, "sentinel.db")
	if err := vacuumInto(ctx, db, snapshot); err != nil {
		return fail(fmt.Errorf("create SQLite backup snapshot: %w", err))
	}
	f, err := os.OpenFile(filepath.Join(stateDir, "state-bundle.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fail(err)
	}
	if err := ExportState(ctx, db, f); err != nil {
		f.Close()
		return fail(err)
	}
	if err := f.Close(); err != nil {
		return fail(err)
	}
	cfgDir := filepath.Join(root, "configs")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		return fail(err)
	}
	for _, src := range configFiles {
		if strings.TrimSpace(src) == "" {
			continue
		}
		info, err := os.Stat(src)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fail(err)
		}
		if info.IsDir() {
			return fail(errors.New("critical config entry must be a file: " + src))
		}
		name := filepath.Base(src)
		if err := copyFile(src, filepath.Join(cfgDir, name), 0o600); err != nil {
			return fail(err)
		}
	}
	if secretRoot != "" {
		dst := filepath.Join(root, "secrets")
		excluded := map[string]bool{}
		for _, x := range excludedSecretNames {
			excluded[x] = true
		}
		if err := copyTree(secretRoot, dst, excluded); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fail(err)
		}
	}
	manifest, err := buildManifest(root)
	if err != nil {
		return fail(err)
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), body, 0o600); err != nil {
		return fail(err)
	}
	return CriticalBundle{Root: root, Cleanup: cleanup}, nil
}
func vacuumInto(ctx context.Context, db *sql.DB, target string) error {
	escaped := strings.ReplaceAll(target, "'", "''")
	_, err := db.ExecContext(ctx, "VACUUM INTO '"+escaped+"'")
	return err
}
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
func copyTree(src, dst string, exclude map[string]bool) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o700)
		}
		if exclude[filepath.Base(path)] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, target, 0o600)
	})
}
func buildManifest(root string) (CriticalManifest, error) {
	m := CriticalManifest{Schema: 1, CreatedAt: time.Now().UTC()}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Base(path) == "manifest.json" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		rel, _ := filepath.Rel(root, path)
		m.Files = append(m.Files, ManifestFile{Path: filepath.ToSlash(rel), SHA256: hex.EncodeToString(h.Sum(nil)), Size: info.Size()})
		return nil
	})
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	return m, err
}
func VerifyCriticalBundle(root string) error {
	body, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return err
	}
	var m CriticalManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return err
	}
	if m.Schema != 1 {
		return errors.New("unsupported critical bundle manifest schema")
	}
	for _, entry := range m.Files {
		clean := filepath.Clean(filepath.FromSlash(entry.Path))
		if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
			return errors.New("unsafe manifest path")
		}
		path := filepath.Join(root, clean)
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		n, copyErr := io.Copy(h, f)
		f.Close()
		if copyErr != nil {
			return copyErr
		}
		if n != entry.Size || hex.EncodeToString(h.Sum(nil)) != entry.SHA256 {
			return fmt.Errorf("critical bundle checksum mismatch: %s", entry.Path)
		}
	}
	return nil
}
