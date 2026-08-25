package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStrictConfigFileAndEnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	body := `{"server":{"listen":"127.0.0.1:9999","read_timeout":"4s"},"database":{"path":"from-file.db"},"security":{"session_ttl":"2h","secure_cookie":false},"features":{"experimental":false}}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SENTINEL_CONFIG", p)
	t.Setenv("SENTINEL_LISTEN", "127.0.0.1:7777")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.ListenAddress != "127.0.0.1:7777" || c.Server.ReadTimeout.String() != "4s" || c.Database.Path != "from-file.db" || c.Security.SessionTTL != 2*time.Hour || c.Security.SecureCookie {
		t.Fatalf("config=%+v", c)
	}
}
func TestStrictConfigRejectsUnknownKey(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(p, []byte(`{"wat":true}`), 0o600)
	t.Setenv("SENTINEL_CONFIG", p)
	if _, err := Load(); err == nil {
		t.Fatal("unknown key accepted")
	}
}
