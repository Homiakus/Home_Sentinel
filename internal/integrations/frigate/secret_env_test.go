package frigate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialDirectorySinkReconcilesOnlySentinelFiles(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "FRIGATE_OTHER"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	s := CredentialDirectorySink{Dir: d}
	if err := s.Materialize(map[string]string{"FRIGATE_SENTINEL_CAM_PASS": "p%40ss"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(d, "FRIGATE_SENTINEL_CAM_PASS"))
	if err != nil || string(b) != "p%40ss" {
		t.Fatalf("b=%q err=%v", b, err)
	}
	if err := s.Materialize(map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(d, "FRIGATE_SENTINEL_CAM_PASS")); !os.IsNotExist(err) {
		t.Fatal("stale Sentinel secret not removed")
	}
	if _, err := os.Stat(filepath.Join(d, "FRIGATE_OTHER")); err != nil {
		t.Fatal("unrelated credential was touched")
	}
}
