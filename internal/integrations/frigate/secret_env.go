package frigate

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SecretEnvSink interface{ Materialize(map[string]string) error }

type FileSecretEnvSink struct{ Path string }

func (s FileSecretEnvSink) Materialize(values map[string]string) error {
	if s.Path == "" {
		return errors.New("Frigate secret env file path required")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		if !validEnvKey(k) {
			return fmt.Errorf("invalid Frigate secret env key %q", k)
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".frigate-env-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	w := bufio.NewWriter(tmp)
	for _, k := range keys {
		v := values[k]
		if strings.ContainsAny(v, "\r\n\x00") {
			tmp.Close()
			return errors.New("Frigate secret env values may not contain line breaks")
		}
		if _, err := fmt.Fprintf(w, "%s=%s\n", k, v); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, s.Path); err != nil {
		return err
	}
	return os.Chmod(s.Path, 0600)
}
func validEnvKey(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(r == '_' || r >= 'A' && r <= 'Z' || (i > 0 && r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// CredentialDirectorySink materializes FRIGATE_* values as Docker credential
// files. Mount Dir read-only into the Frigate container at /run/secrets (or set
// CREDENTIALS_DIRECTORY accordingly). Only files with our Sentinel prefix are
// reconciled; unrelated credentials are never touched.
type CredentialDirectorySink struct{ Dir string }

func (s CredentialDirectorySink) Materialize(values map[string]string) error {
	if s.Dir == "" {
		return errors.New("Frigate credentials directory required")
	}
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return err
	}
	wanted := map[string]bool{}
	for k, v := range values {
		if !validEnvKey(k) || !strings.HasPrefix(k, "FRIGATE_SENTINEL_") {
			return fmt.Errorf("invalid Sentinel Frigate credential key %q", k)
		}
		if strings.ContainsAny(v, "\r\n\x00") {
			return errors.New("Frigate credential values may not contain line breaks")
		}
		wanted[k] = true
		tmp, err := os.CreateTemp(s.Dir, ".credential-*")
		if err != nil {
			return err
		}
		name := tmp.Name()
		if err := tmp.Chmod(0400); err != nil {
			tmp.Close()
			os.Remove(name)
			return err
		}
		if _, err := tmp.WriteString(v); err != nil {
			tmp.Close()
			os.Remove(name)
			return err
		}
		if err := tmp.Sync(); err != nil {
			tmp.Close()
			os.Remove(name)
			return err
		}
		if err := tmp.Close(); err != nil {
			os.Remove(name)
			return err
		}
		dst := filepath.Join(s.Dir, k)
		if err := os.Rename(name, dst); err != nil {
			os.Remove(name)
			return err
		}
		if err := os.Chmod(dst, 0400); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "FRIGATE_SENTINEL_") {
			continue
		}
		if !wanted[e.Name()] {
			if err := os.Remove(filepath.Join(s.Dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
