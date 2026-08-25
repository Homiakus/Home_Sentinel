package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvProvider(t *testing.T) {
	t.Setenv("SENTINEL_TEST_SECRET", "value")
	r, _ := ParseRef("secret://env/SENTINEL_TEST_SECRET")
	b, err := (EnvProvider{}).Resolve(r)
	if err != nil || string(b) != "value" {
		t.Fatalf("b=%q err=%v", b, err)
	}
}
func TestFileProviderRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "x"), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, _ := ParseRef("secret://file/x")
	b, err := (FileProvider{Root: root}).Resolve(r)
	if err != nil || string(b) != "secret" {
		t.Fatalf("b=%q err=%v", b, err)
	}
	bad, _ := ParseRef("secret://file/../escape")
	if _, err := (FileProvider{Root: root}).Resolve(bad); err == nil {
		t.Fatal("expected traversal rejection")
	}
}
