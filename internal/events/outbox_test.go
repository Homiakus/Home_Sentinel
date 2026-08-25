package events

import (
	"context"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"path/filepath"
	"testing"
	"time"
)

func TestOutboxCrashLeaseAndAck(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "o.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	o := Outbox{DB: db}
	m, err := o.Enqueue(ctx, "telegram", []byte(`{"x":1}`))
	if err != nil {
		t.Fatal(err)
	}
	first, err := o.Claim(ctx, 10, 20*time.Millisecond)
	if err != nil || len(first) != 1 || first[0].ID != m.ID {
		t.Fatalf("first=%v err=%v", first, err)
	}
	again, _ := o.Claim(ctx, 10, time.Second)
	if len(again) != 0 {
		t.Fatal("message reclaimed before lease")
	}
	time.Sleep(30 * time.Millisecond)
	again, err = o.Claim(ctx, 10, time.Second)
	if err != nil || len(again) != 1 || again[0].Attempts != 2 {
		t.Fatalf("again=%v err=%v", again, err)
	}
	if err := o.Ack(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	none, _ := o.Claim(ctx, 10, time.Second)
	if len(none) != 0 {
		t.Fatal("delivered reclaimed")
	}
}
