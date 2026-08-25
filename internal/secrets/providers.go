package secrets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type EnvProvider struct{}

func (EnvProvider) Resolve(ref Ref) ([]byte, error) {
	const p = "secret://env/"
	raw := ref.String()
	if !strings.HasPrefix(raw, p) {
		return nil, errors.New("not an env secret reference")
	}
	name := strings.TrimPrefix(raw, p)
	if name == "" || strings.ContainsAny(name, "/=\\") {
		return nil, errors.New("invalid environment secret name")
	}
	v, ok := os.LookupEnv(name)
	if !ok {
		return nil, fmt.Errorf("environment secret %s not set", name)
	}
	return []byte(v), nil
}

type FileProvider struct{ Root string }

func (p FileProvider) Resolve(ref Ref) ([]byte, error) {
	const prefix = "secret://file/"
	raw := ref.String()
	if !strings.HasPrefix(raw, prefix) {
		return nil, errors.New("not a file secret reference")
	}
	rel := strings.TrimPrefix(raw, prefix)
	if rel == "" {
		return nil, errors.New("empty file secret path")
	}
	root, err := filepath.Abs(p.Root)
	if err != nil {
		return nil, err
	}
	candidate := rel
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, filepath.FromSlash(rel))
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return nil, err
	}
	r, err := filepath.Rel(root, candidate)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return nil, errors.New("file secret escapes allowed root")
	}
	b, err := os.ReadFile(candidate)
	if err != nil {
		return nil, err
	}
	return []byte(strings.TrimRight(string(b), "\r\n")), nil
}

type Resolver struct {
	Env  EnvProvider
	File FileProvider
}

func (r Resolver) Resolve(ref Ref) ([]byte, error) {
	switch {
	case strings.HasPrefix(ref.String(), "secret://env/"):
		return r.Env.Resolve(ref)
	case strings.HasPrefix(ref.String(), "secret://file/"):
		return r.File.Resolve(ref)
	default:
		return nil, fmt.Errorf("unsupported secret provider in %q", ref.String())
	}
}
