package health

import (
	"sort"
	"sync"
	"time"
)

type Registry struct {
	mu         sync.RWMutex
	components map[string]Component
}

func NewRegistry() *Registry { return &Registry{components: map[string]Component{}} }

func (r *Registry) Set(name string, status Status, reason, cause string) {
	if r == nil || name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	prev, ok := r.components[name]
	now := time.Now().UTC()
	if !ok || prev.Status != status || prev.ReasonCode != reason || prev.Cause != cause {
		prev.Since = now
	}
	prev.Name, prev.Status, prev.ReasonCode, prev.Cause = name, status, reason, cause
	r.components[name] = prev
}

func (r *Registry) Get(name string) (Component, bool) {
	if r == nil {
		return Component{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.components[name]
	return v, ok
}

func (r *Registry) Snapshot() []Component {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Component, 0, len(r.components))
	for _, v := range r.components {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *Registry) Overall() Status {
	items := r.Snapshot()
	if len(items) == 0 {
		return Unknown
	}
	worst := Healthy
	for _, c := range items {
		switch c.Status {
		case Failed:
			return Failed
		case Degraded:
			worst = Degraded
		case Starting:
			if worst == Healthy {
				worst = Starting
			}
		case Unknown:
			if worst == Healthy {
				worst = Unknown
			}
		}
	}
	return worst
}

type DependencyGraph map[string][]string

type Diagnosis struct {
	Component    Component `json:"component"`
	RootCause    bool      `json:"root_cause"`
	SuppressedBy string    `json:"suppressed_by,omitempty"`
}

func Diagnose(reg *Registry, graph DependencyGraph) []Diagnosis {
	items := reg.Snapshot()
	byName := make(map[string]Component, len(items))
	for _, c := range items {
		byName[c.Name] = c
	}
	out := make([]Diagnosis, 0, len(items))
	for _, c := range items {
		d := Diagnosis{Component: c}
		if c.Status == Failed || c.Status == Degraded {
			for _, dep := range graph[c.Name] {
				if dc, ok := byName[dep]; ok && (dc.Status == Failed || dc.Status == Degraded) {
					d.SuppressedBy = dep
					break
				}
			}
			d.RootCause = d.SuppressedBy == ""
		}
		out = append(out, d)
	}
	return out
}
