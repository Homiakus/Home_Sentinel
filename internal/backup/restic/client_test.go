package restic

import (
	"context"
	"os"
	"strings"
	"testing"
)

type captureRunner struct {
	args []string
	env  []string
	out  []byte
}

func (r *captureRunner) Run(_ context.Context, _ string, args []string, env []string) ([]byte, []byte, error) {
	r.args = append([]string(nil), args...)
	r.env = append([]string(nil), env...)
	return append([]byte(nil), r.out...), nil, nil
}

func TestPasswordNeverAppearsInArguments(t *testing.T) {
	r := &captureRunner{out: []byte(`{"message_type":"summary","snapshot_id":"abc123"}` + "\n")}
	c := Client{Binary: "restic", Repository: "/repo", Password: []byte("super-secret-password"), Runner: r, TempDir: t.TempDir()}
	res, err := c.Backup(context.Background(), []string{"/data"}, []string{"sentinel"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.SnapshotID != "abc123" {
		t.Fatalf("snapshot=%q", res.SnapshotID)
	}
	joined := strings.Join(r.args, " ")
	if strings.Contains(joined, "super-secret-password") {
		t.Fatal("password leaked to argv")
	}
	var passwordFile string
	for _, e := range r.env {
		if strings.HasPrefix(e, "RESTIC_PASSWORD_FILE=") {
			passwordFile = strings.TrimPrefix(e, "RESTIC_PASSWORD_FILE=")
		}
	}
	if passwordFile == "" {
		t.Fatal("RESTIC_PASSWORD_FILE missing")
	}
	if _, err := os.Stat(passwordFile); !os.IsNotExist(err) {
		t.Fatalf("temporary password file should be deleted, stat err=%v", err)
	}
}

func TestRestoreArguments(t *testing.T) {
	r := &captureRunner{}
	c := Client{Repository: "/repo", Password: []byte("pw"), Runner: r, TempDir: t.TempDir()}
	if err := c.Restore(context.Background(), "snap", "/restore"); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(r.args, " ")
	if got != "restore snap --target /restore" {
		t.Fatalf("args=%q", got)
	}
}
