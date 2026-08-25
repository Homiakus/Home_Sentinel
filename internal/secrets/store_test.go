package secrets

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFileStorePutResolveDelete(t *testing.T) {
	root := t.TempDir()
	s := FileStore{Root: root}
	ref, err := s.Put("ha-token", []byte("top-secret"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := (FileProvider{Root: root}).Resolve(ref)
	if err != nil || string(b) != "top-secret" {
		t.Fatalf("b=%q err=%v", b, err)
	}
	st, err := os.Stat(filepath.Join(root, "ha-token"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", st.Mode().Perm())
	}
	if err := s.Delete("ha-token"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "ha-token")); !os.IsNotExist(err) {
		t.Fatalf("expected deleted: %v", err)
	}
}
