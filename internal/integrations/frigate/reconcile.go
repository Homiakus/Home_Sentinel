package frigate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	fgconfig "github.com/Homiakus/Home_Sentinel/internal/integrations/frigate/config"
)

type DriftKind string

const (
	DriftMissing    DriftKind = "missing"
	DriftChanged    DriftKind = "changed"
	DriftUnexpected DriftKind = "unexpected"
)

type Drift struct {
	Kind            DriftKind `json:"kind"`
	Resource        string    `json:"resource"`
	DesiredChecksum string    `json:"desired_checksum,omitempty"`
	ActualChecksum  string    `json:"actual_checksum,omitempty"`
}
type ReconcileReport struct {
	InSync bool    `json:"in_sync"`
	Drift  []Drift `json:"drift,omitempty"`
}

func Reconcile(desiredJSON []byte, actual map[string]any, ownership fgconfig.Ownership) ReconcileReport {
	var desired map[string]any
	if json.Unmarshal(desiredJSON, &desired) != nil {
		return ReconcileReport{InSync: false, Drift: []Drift{{Kind: DriftChanged, Resource: "config:invalid-desired"}}}
	}
	out := ReconcileReport{InSync: true}
	compareGroup := func(prefix string, names []string, dgroup, agroup map[string]any) {
		sort.Strings(names)
		for _, name := range names {
			dv, dok := dgroup[name]
			av, aok := agroup[name]
			switch {
			case dok && !aok:
				out.Drift = append(out.Drift, Drift{Kind: DriftMissing, Resource: prefix + name, DesiredChecksum: checksum(dv)})
			case dok && aok && checksum(dv) != checksum(av):
				out.Drift = append(out.Drift, Drift{Kind: DriftChanged, Resource: prefix + name, DesiredChecksum: checksum(dv), ActualChecksum: checksum(av)})
			case !dok && aok:
				out.Drift = append(out.Drift, Drift{Kind: DriftUnexpected, Resource: prefix + name, ActualChecksum: checksum(av)})
			}
		}
	}
	compareGroup("camera:", append([]string(nil), ownership.CameraNames...), mapAt(desired, "cameras"), mapAt(actual, "cameras"))
	compareGroup("stream:", append([]string(nil), ownership.StreamNames...), nestedMap(desired, "go2rtc", "streams"), nestedMap(actual, "go2rtc", "streams"))
	out.InSync = len(out.Drift) == 0
	return out
}
func checksum(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func mapAt(m map[string]any, k string) map[string]any {
	if x, ok := m[k].(map[string]any); ok {
		return x
	}
	return map[string]any{}
}
func nestedMap(m map[string]any, a, b string) map[string]any { return mapAt(mapAt(m, a), b) }
