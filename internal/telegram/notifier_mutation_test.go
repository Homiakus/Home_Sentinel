package telegram

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
)

type scriptedNotificationDeliveryStore struct {
	claims     []notificationDelivery
	claimFlags []bool
	claimErrs  []error
	claimCalls int
}

func (s *scriptedNotificationDeliveryStore) Freeze(context.Context, gateway.Operation, string, []notificationRecipient) (notificationOperation, bool, error) {
	return notificationOperation{}, false, errors.New("unexpected Freeze")
}
func (s *scriptedNotificationDeliveryStore) Load(context.Context, string) (notificationOperation, error) {
	return notificationOperation{}, errors.New("unexpected Load")
}
func (s *scriptedNotificationDeliveryStore) ClaimPrepared(context.Context, string, int64) (notificationDelivery, bool, error) {
	i := s.claimCalls
	s.claimCalls++
	if i < len(s.claimErrs) && s.claimErrs[i] != nil {
		return notificationDelivery{}, false, s.claimErrs[i]
	}
	var delivery notificationDelivery
	if i < len(s.claims) {
		delivery = s.claims[i]
	}
	claimed := i < len(s.claimFlags) && s.claimFlags[i]
	return delivery, claimed, nil
}
func (s *scriptedNotificationDeliveryStore) MarkApplied(context.Context, string, int64, int64) error {
	return errors.New("unexpected MarkApplied")
}
func (s *scriptedNotificationDeliveryStore) MarkAmbiguous(context.Context, string, int64) error {
	return errors.New("unexpected MarkAmbiguous")
}
func (s *scriptedNotificationDeliveryStore) ReleasePrepared(context.Context, string, int64) error {
	return errors.New("unexpected ReleasePrepared")
}

func TestDurableNotifierRejectsZeroTelegramIdentityBeforeUserLookup(t *testing.T) {
	pairings := &fakeNotificationPairings{bindings: []Binding{{TelegramUserID: 0, UserID: "admin", ChatID: 10}}}
	users := &fakeNotificationUsers{users: map[string]auth.User{"admin": {ID: "admin", Role: auth.RoleAdmin}}, errs: map[string]error{}}
	n := &DurableNotifier{Pairings: pairings, Users: users}
	if _, err := n.selectRecipients(context.Background()); err == nil {
		t.Fatal("zero Telegram identity accepted")
	}
	if users.callCount() != 0 {
		t.Fatalf("malformed identity reached user lookup: %d", users.callCount())
	}
}

func TestDurableNotifierRecipientOrderingIsCanonical(t *testing.T) {
	pairings := &fakeNotificationPairings{bindings: []Binding{
		{TelegramUserID: 20, UserID: "admin", ChatID: 20},
		{TelegramUserID: 10, UserID: "operator", ChatID: 10},
	}}
	users := &fakeNotificationUsers{users: map[string]auth.User{
		"admin":    {ID: "admin", Role: auth.RoleAdmin},
		"operator": {ID: "operator", Role: auth.RoleOperator},
	}, errs: map[string]error{}}
	n := &DurableNotifier{Pairings: pairings, Users: users}
	recipients, err := n.selectRecipients(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 2 || recipients[0].TelegramUserID != 10 || recipients[1].TelegramUserID != 20 {
		t.Fatalf("recipient order=%+v", recipients)
	}
}

func TestClaimPreparedBoundAndConvergenceBranches(t *testing.T) {
	prepared := notificationDelivery{State: deliveryPrepared}
	applied := notificationDelivery{State: deliveryApplied}

	t.Run("bounded contention", func(t *testing.T) {
		store := &scriptedNotificationDeliveryStore{claims: []notificationDelivery{prepared, prepared, prepared}}
		n := &DurableNotifier{Deliveries: store}
		_, claimed, err := n.claimPrepared(context.Background(), "key", 1)
		if err == nil || claimed {
			t.Fatalf("claim result claimed=%v err=%v", claimed, err)
		}
		if store.claimCalls != maxClaimAttempts {
			t.Fatalf("claim calls=%d want=%d", store.claimCalls, maxClaimAttempts)
		}
	})

	t.Run("claim winner returns immediately", func(t *testing.T) {
		store := &scriptedNotificationDeliveryStore{claims: []notificationDelivery{prepared}, claimFlags: []bool{true}}
		n := &DurableNotifier{Deliveries: store}
		delivery, claimed, err := n.claimPrepared(context.Background(), "key", 1)
		if err != nil || !claimed || delivery.State != deliveryPrepared || store.claimCalls != 1 {
			t.Fatalf("delivery=%+v claimed=%v calls=%d err=%v", delivery, claimed, store.claimCalls, err)
		}
	})

	t.Run("other caller progressed state", func(t *testing.T) {
		store := &scriptedNotificationDeliveryStore{claims: []notificationDelivery{applied}}
		n := &DurableNotifier{Deliveries: store}
		delivery, claimed, err := n.claimPrepared(context.Background(), "key", 1)
		if err != nil || claimed || delivery.State != deliveryApplied || store.claimCalls != 1 {
			t.Fatalf("delivery=%+v claimed=%v calls=%d err=%v", delivery, claimed, store.claimCalls, err)
		}
	})

	t.Run("store error stops retries", func(t *testing.T) {
		boom := errors.New("claim failed")
		store := &scriptedNotificationDeliveryStore{claimErrs: []error{boom}}
		n := &DurableNotifier{Deliveries: store}
		_, _, err := n.claimPrepared(context.Background(), "key", 1)
		if !errors.Is(err, boom) || store.claimCalls != 1 {
			t.Fatalf("calls=%d err=%v", store.claimCalls, err)
		}
	})
}

func TestCanonicalizeNotificationExactTelegramLimitAccepted(t *testing.T) {
	body := strings.Repeat("я", telegramTextLimit)
	canonical, _, err := canonicalizeNotification(gateway.Notification{Channel: "owner", Body: body})
	if err != nil {
		t.Fatalf("exact Telegram text limit rejected: %v", err)
	}
	if canonical.Text != body {
		t.Fatal("canonical text changed at exact limit")
	}
	if _, _, err := canonicalizeNotification(gateway.Notification{Channel: "owner", Body: body + "я"}); err == nil {
		t.Fatal("Telegram text limit+1 accepted")
	}
}

func TestSameNotificationRecipientsMismatchMatrix(t *testing.T) {
	r1 := notificationRecipient{TelegramUserID: 1, UserID: "one", ChatID: 11}
	r2 := notificationRecipient{TelegramUserID: 2, UserID: "two", ChatID: 22}
	changed := notificationRecipient{TelegramUserID: 2, UserID: "changed", ChatID: 22}
	if !sameNotificationRecipients([]notificationDelivery{{Recipient: r1}, {Recipient: r2}}, []notificationRecipient{r1, r2}) {
		t.Fatal("exact recipient set did not match")
	}
	if sameNotificationRecipients([]notificationDelivery{{Recipient: r1}}, []notificationRecipient{r1, r2}) {
		t.Fatal("length mismatch accepted")
	}
	if sameNotificationRecipients([]notificationDelivery{{Recipient: r1}, {Recipient: r2}}, []notificationRecipient{r1, changed}) {
		t.Fatal("semantic recipient mismatch accepted")
	}
}

func TestSummarizeFrozenNotificationProviderIDBoundary(t *testing.T) {
	zero := notificationOperation{IdempotencyKey: "zero", Deliveries: []notificationDelivery{{
		State: deliveryApplied, ProviderMessageID: 0,
	}}}
	result, done := summarizeFrozenNotification(zero, false)
	if !done || result.State != gateway.EffectAlreadyApplied || result.ProviderID != "" {
		t.Fatalf("zero provider summary=%+v done=%v", result, done)
	}
	one := zero
	one.Deliveries = []notificationDelivery{{State: deliveryApplied, ProviderMessageID: 1}}
	result, done = summarizeFrozenNotification(one, false)
	if !done || result.ProviderID != "1" {
		t.Fatalf("positive provider summary=%+v done=%v", result, done)
	}
}

func TestNotificationDeliveryStoreZeroProviderIDRejectedBeforeTransition(t *testing.T) {
	db := openNotificationDB(t, filepath.Join(t.TempDir(), "zero-provider.db"))
	defer db.Close()
	store := NotificationDeliveryStore{DB: db}
	op := gateway.Operation{ExecutionID: "incident-zero", IdempotencyKey: "notify-zero"}
	if _, _, err := store.Freeze(context.Background(), op, "digest", []notificationRecipient{{TelegramUserID: 1, UserID: "admin", ChatID: 1}}); err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := store.ClaimPrepared(context.Background(), op.IdempotencyKey, 1); err != nil || !claimed {
		t.Fatalf("claim claimed=%v err=%v", claimed, err)
	}
	if err := store.MarkApplied(context.Background(), op.IdempotencyKey, 1, 0); err == nil {
		t.Fatal("zero provider id transitioned sending->applied")
	}
	loaded, err := store.Load(context.Background(), op.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Deliveries[0].State != deliverySending {
		t.Fatalf("zero provider id changed state to %q", loaded.Deliveries[0].State)
	}
}

func TestNotificationDeliveryStoreNonAppliedProviderIDRemainsNull(t *testing.T) {
	db := openNotificationDB(t, filepath.Join(t.TempDir(), "provider-null.db"))
	defer db.Close()
	store := NotificationDeliveryStore{DB: db}
	prepare := func(key string, id int64) {
		op := gateway.Operation{ExecutionID: "incident-" + key, IdempotencyKey: key}
		if _, _, err := store.Freeze(context.Background(), op, "digest-"+key, []notificationRecipient{{TelegramUserID: id, UserID: "admin", ChatID: id}}); err != nil {
			t.Fatal(err)
		}
		if _, claimed, err := store.ClaimPrepared(context.Background(), key, id); err != nil || !claimed {
			t.Fatalf("claim %s claimed=%v err=%v", key, claimed, err)
		}
	}
	prepare("ambiguous-null", 1)
	if err := store.MarkAmbiguous(context.Background(), "ambiguous-null", 1); err != nil {
		t.Fatal(err)
	}
	prepare("prepared-null", 2)
	if err := store.ReleasePrepared(context.Background(), "prepared-null", 2); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ambiguous-null", "prepared-null"} {
		var provider sql.NullInt64
		if err := db.QueryRowContext(context.Background(), `SELECT provider_message_id FROM telegram_notification_deliveries WHERE idempotency_key=?`, key).Scan(&provider); err != nil {
			t.Fatal(err)
		}
		if provider.Valid {
			t.Fatalf("non-applied delivery %q stored provider id %d", key, provider.Int64)
		}
	}
}
