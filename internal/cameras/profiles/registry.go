package profiles

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
)

//go:embed defaults/*.json
var defaults embed.FS

type Hint struct {
	Name string             `json:"name"`
	Path string             `json:"path"`
	Role cameras.StreamRole `json:"role"`
}
type Profile struct {
	Schema            int      `json:"schema"`
	ID                string   `json:"id"`
	ManufacturerMatch []string `json:"manufacturer_match"`
	RTSPHints         []Hint   `json:"rtsp_hints"`
}
type Registry struct{ profiles []Profile }

func LoadDefault() (Registry, error) { return loadFS(defaults, "defaults") }
func LoadWithOverrides(dir string) (Registry, error) {
	r, err := LoadDefault()
	if err != nil {
		return Registry{}, err
	}
	if strings.TrimSpace(dir) == "" {
		return r, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return Registry{}, err
	}
	byID := map[string]Profile{}
	for _, p := range r.profiles {
		byID[p.ID] = p
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return Registry{}, err
		}
		p, err := decode(body)
		if err != nil {
			return Registry{}, fmt.Errorf("camera profile %s: %w", e.Name(), err)
		}
		byID[p.ID] = p
	}
	r.profiles = r.profiles[:0]
	for _, p := range byID {
		r.profiles = append(r.profiles, p)
	}
	sort.Slice(r.profiles, func(i, j int) bool { return r.profiles[i].ID < r.profiles[j].ID })
	return r, nil
}
func loadFS(fsys fs.FS, dir string) (Registry, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return Registry{}, err
	}
	var r Registry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		body, err := fs.ReadFile(fsys, dir+"/"+e.Name())
		if err != nil {
			return Registry{}, err
		}
		p, err := decode(body)
		if err != nil {
			return Registry{}, fmt.Errorf("%s: %w", e.Name(), err)
		}
		r.profiles = append(r.profiles, p)
	}
	sort.Slice(r.profiles, func(i, j int) bool { return r.profiles[i].ID < r.profiles[j].ID })
	return r, nil
}
func decode(body []byte) (Profile, error) {
	var p Profile
	if err := json.Unmarshal(body, &p); err != nil {
		return Profile{}, err
	}
	if p.Schema != 1 || p.ID == "" {
		return Profile{}, errors.New("invalid profile schema/id")
	}
	for _, h := range p.RTSPHints {
		if !strings.HasPrefix(h.Path, "/") {
			return Profile{}, errors.New("RTSP hint path must begin with /")
		}
		if h.Role != cameras.RoleMain && h.Role != cameras.RoleDetect {
			return Profile{}, errors.New("invalid RTSP hint role")
		}
	}
	return p, nil
}
func (r Registry) Match(manufacturer string) Profile {
	m := strings.ToLower(strings.TrimSpace(manufacturer))
	var generic Profile
	for _, p := range r.profiles {
		for _, pattern := range p.ManufacturerMatch {
			pattern = strings.ToLower(strings.TrimSpace(pattern))
			if pattern == "*" {
				generic = p
				continue
			}
			if pattern != "" && strings.Contains(m, pattern) {
				return p
			}
		}
	}
	return generic
}
func (r Registry) All() []Profile { return append([]Profile(nil), r.profiles...) }
