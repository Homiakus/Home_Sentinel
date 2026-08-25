package backup

type SetClass string

const (
	Critical   SetClass = "critical"
	Important  SetClass = "important"
	Disposable SetClass = "disposable"
)

type BackupSet struct {
	Name     string   `json:"name"`
	Class    SetClass `json:"class"`
	Paths    []string `json:"paths"`
	Tags     []string `json:"tags"`
	Excludes []string `json:"excludes,omitempty"`
}

func DefaultCriticalSet(stagingRoot string) BackupSet {
	return BackupSet{Name: "sentinel-critical", Class: Critical, Paths: []string{stagingRoot}, Tags: []string{"sentinel", "critical"}}
}
