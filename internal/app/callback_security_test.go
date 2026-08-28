package app

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/config"
	"github.com/Homiakus/Home_Sentinel/internal/secrets"
	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
)

func TestOpenLeavesCallbackAuthorityDisabledByDefault(t *testing.T) {
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "sentinel.db")
	a, err := Open(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.CallbackSecurity != nil {
		t.Fatal("callback authority initialized while callbacks are disabled")
	}
}

func TestOpenFailsClosedWhenCallbackSecretCannotBeResolved(t *testing.T) {
	const envName = "SENTINEL_TEST_CALLBACK_KEY_MISSING"
	t.Setenv(envName, "")

	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "sentinel.db")
	cfg.Security.Callbacks.Enabled = true
	cfg.Security.Callbacks.ActiveKeyID = "active"
	cfg.Security.Callbacks.Keys = map[string]secrets.Ref{
		"active": secrets.Ref("secret://env/" + envName),
	}
	_, err := Open(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("startup accepted unresolved callback secret")
	}
	if !strings.Contains(err.Error(), "initialize callback security") {
		t.Fatalf("startup returned unrelated error: %v", err)
	}
}

func TestOpenInitializesCallbackAuthorityFromSecretReference(t *testing.T) {
	const envName = "SENTINEL_TEST_CALLBACK_KEY"
	t.Setenv(envName, strings.Repeat("k", callback.MinKeyBytes))

	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "sentinel.db")
	cfg.Security.Callbacks.Enabled = true
	cfg.Security.Callbacks.ActiveKeyID = "active"
	cfg.Security.Callbacks.Keys = map[string]secrets.Ref{
		"active": secrets.Ref("secret://env/" + envName),
	}
	a, err := Open(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.CallbackSecurity == nil {
		t.Fatal("callback authority was not initialized")
	}

	claims := callback.Claims{
		ExecutionID: "incident-runtime-test",
		NodeID:      "await-owner-ack",
		EventID:     "owner-response-1",
		Action:      "incident.owner.response",
		Nonce:       "runtime-nonce-1",
		ExpiresAt:   time.Now().UTC().Add(time.Minute).Unix(),
	}
	token, err := a.CallbackSecurity.Sign(claims)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := a.CallbackSecurity.Accept(token, callback.Binding{
		ExecutionID: claims.ExecutionID,
		NodeID:      claims.NodeID,
		EventID:     claims.EventID,
		Action:      claims.Action,
	})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.KeyID != "active" {
		t.Fatalf("accepted key id=%q want active", accepted.KeyID)
	}
}
