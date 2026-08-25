package backup

import (
	"context"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckpointExportRestore(t *testing.T) {
	ctx := context.Background()
	dbp := filepath.Join(t.TempDir(), "s.db")
	db, e := database.Open(ctx, database.Options{Path: dbp, BusyTimeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	if e = database.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO resources(kind,id,created_at,updated_at,revision,payload) VALUES('x','1','a','a',1,'{}')`); e != nil {
		t.Fatal(e)
	}
	cp := filepath.Join(t.TempDir(), "cp")
	if e = ExportCriticalToDirectory(ctx, db, nil, "", cp); e != nil {
		t.Fatal(e)
	}
	db.Close()
	if e = os.WriteFile(dbp, []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = RestoreCriticalFromDirectory(cp, dbp, ""); e != nil {
		t.Fatal(e)
	}
	db, e = database.Open(ctx, database.Options{Path: dbp, BusyTimeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var n int
	if e = db.QueryRow(`SELECT COUNT(*) FROM resources`).Scan(&n); e != nil || n != 1 {
		t.Fatalf("restore count=%d err=%v", n, e)
	}
}
