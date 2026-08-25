package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Component struct {
	Ref      string `json:"ref"`
	Required bool   `json:"required"`
}
type Manifest struct {
	Version           string               `json:"version"`
	MinCurrentVersion string               `json:"min_current_version,omitempty"`
	SchemaTarget      int64                `json:"schema_target"`
	MinReadableSchema int64                `json:"min_readable_schema"`
	Irreversible      bool                 `json:"irreversible"`
	Components        map[string]Component `json:"components"`
	Notes             string               `json:"notes,omitempty"`
}
type Current struct {
	Version       string            `json:"version"`
	SchemaVersion int64             `json:"schema_version"`
	Components    map[string]string `json:"components"`
}
type Action struct {
	Component string `json:"component"`
	From      string `json:"from"`
	To        string `json:"to"`
	Changed   bool   `json:"changed"`
}
type Plan struct {
	FromVersion    string   `json:"from_version"`
	ToVersion      string   `json:"to_version"`
	FromSchema     int64    `json:"from_schema"`
	ToSchema       int64    `json:"to_schema"`
	Compatible     bool     `json:"compatible"`
	Reasons        []string `json:"reasons,omitempty"`
	Actions        []Action `json:"actions"`
	BackupRequired bool     `json:"backup_required"`
	Irreversible   bool     `json:"irreversible"`
}

var semverRE = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$`)

func semver(raw string) ([3]int, error) {
	m := semverRE.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return [3]int{}, fmt.Errorf("invalid semantic version %q", raw)
	}
	var v [3]int
	for i := 0; i < 3; i++ {
		n, _ := strconv.Atoi(m[i+1])
		v[i] = n
	}
	return v, nil
}
func compareVersion(a, b string) (int, error) {
	aa, e := semver(a)
	if e != nil {
		return 0, e
	}
	bb, e := semver(b)
	if e != nil {
		return 0, e
	}
	for i := 0; i < 3; i++ {
		if aa[i] < bb[i] {
			return -1, nil
		}
		if aa[i] > bb[i] {
			return 1, nil
		}
	}
	return 0, nil
}
func ExactRef(ref string) bool {
	r := strings.TrimSpace(ref)
	if r == "" || strings.Contains(strings.ToLower(r), ":latest") || strings.HasSuffix(strings.ToLower(r), "/latest") {
		return false
	}
	if strings.Contains(r, "@sha256:") {
		return true
	}
	slash := strings.LastIndex(r, "/")
	colon := strings.LastIndex(r, ":")
	return colon > slash && colon < len(r)-1
}
func (m Manifest) Validate() error {
	if _, e := semver(m.Version); e != nil {
		return e
	}
	if m.SchemaTarget <= 0 || m.MinReadableSchema <= 0 {
		return errors.New("schema versions must be positive")
	}
	if m.MinReadableSchema > m.SchemaTarget {
		return errors.New("min readable schema cannot exceed target")
	}
	if len(m.Components) == 0 {
		return errors.New("release components required")
	}
	for n, c := range m.Components {
		if strings.TrimSpace(n) == "" || !ExactRef(c.Ref) {
			return fmt.Errorf("component %q must use an exact non-latest tag or digest", n)
		}
	}
	return nil
}
func ParseManifest(b []byte) (Manifest, error) {
	var m Manifest
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if e := d.Decode(&m); e != nil {
		return m, e
	}
	return m, m.Validate()
}
func BuildPlan(cur Current, m Manifest) (Plan, error) {
	if e := m.Validate(); e != nil {
		return Plan{}, e
	}
	p := Plan{FromVersion: cur.Version, ToVersion: m.Version, FromSchema: cur.SchemaVersion, ToSchema: m.SchemaTarget, Compatible: true, BackupRequired: true, Irreversible: m.Irreversible}
	if cur.SchemaVersion > m.SchemaTarget {
		p.Compatible = false
		p.Reasons = append(p.Reasons, "database schema is newer than target release")
	}
	if cur.SchemaVersion < m.MinReadableSchema {
		p.Compatible = false
		p.Reasons = append(p.Reasons, "current database schema is older than release migration window")
	}
	if m.MinCurrentVersion != "" {
		cmp, e := compareVersion(cur.Version, m.MinCurrentVersion)
		if e != nil {
			return Plan{}, e
		}
		if cmp < 0 {
			p.Compatible = false
			p.Reasons = append(p.Reasons, "current version is below release minimum")
		}
	}
	names := make([]string, 0, len(m.Components))
	for n := range m.Components {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		to := m.Components[n].Ref
		from := cur.Components[n]
		p.Actions = append(p.Actions, Action{Component: n, From: from, To: to, Changed: from != to})
	}
	return p, nil
}
