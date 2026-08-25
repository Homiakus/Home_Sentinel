package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var secretName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

type FileStore struct{ Root string }

func (s FileStore) Put(name string, value []byte) (Ref, error) {
	if !secretName.MatchString(name) {
		return "", errors.New("invalid secret name")
	}
	if len(value) == 0 {
		return "", errors.New("secret value required")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	_ = os.Chmod(root, 0o700)
	tmp, err := os.CreateTemp(root, ".sentinel-secret-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := tmp.Write(value); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	target := filepath.Join(root, name)
	if err := os.Rename(tmpName, target); err != nil {
		return "", err
	}
	if err := os.Chmod(target, 0o600); err != nil {
		return "", err
	}
	return ParseRef("secret://file/" + name)
}

func (s FileStore) Delete(name string) error {
	if !secretName.MatchString(name) {
		return errors.New("invalid secret name")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(root, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s FileStore) DeleteRef(ref Ref) error {
	const prefix = "secret://file/"
	raw := ref.String()
	if !strings.HasPrefix(raw, prefix) {
		return errors.New("secret reference is not managed by file store")
	}
	return s.Delete(strings.TrimPrefix(raw, prefix))
}
