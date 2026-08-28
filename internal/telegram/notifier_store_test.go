package telegram

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/gateway"
)

func TestNotificationDeliveryStoreFreezeIsTransactionalAndIdempotent(t *testing.T) {
	db := openNotificationDB(t, filepath.Join(t.TempDir(), "store.db"))
	defer db.Close()
	store := NotificationDeliveryStore{DB: db}
	op := gateway.Operation{ExecutionID: "incident-store", IdempotencyKey: "notify-store"}
	recipients := []notificationRecipient{
		{TelegramUserID: 2, UserID: "admin", ChatID: 802},
		{TelegramUserID: 1, UserID: "operator", ChatID: 801},
	}

	frozen, created, err := store.Freeze(context.Background(), op, "digest-1", recipients)
	if err != nil || !created {
		t.Fatalf("first freeze created=%v err=%v", created, err)
	}
	if frozen.ExecutionID != op.ExecutionID || frozen.SemanticDigest != "digest-1" || len(frozen.Deliveries) != 2 {
		t.Fatalf("frozen=%+v", frozen)
	}
	// Store load order is deterministic by Telegram user id, independently of
	// caller input order.
	if frozen.Deliveries[0].Recipient.TelegramUserID != 1 || frozen.Deliveries[1].Recipient.TelegramUserID != 2 {
		t.Fatalf("delivery order=%+v", frozen.Deliveries)
	}

	replay, created, err := store.Freeze(context.Background(), op, "digest-1", recipients)
	if err != nil || created {
		t.Fatalf("replay freeze created=%v err=%v", created, err)
	}
	if len(replay.Deliveries) != 2 {
		t.Fatalf("replay deliveries=%d", len(replay.Deliveries))
	}

	bad := gateway.Operation{ExecutionID: "incident-rollback", IdempotencyKey: "notify-rollback"}
	_, _, err = store.Freeze(context.Background(), bad, "digest-2", []notificationRecipient{
		{TelegramUserID: 3, UserID: "admin", ChatID: 803},
		{TelegramUserID: 0, UserID: "invalid", ChatID: 804},
	})
	if err == nil {
		t.Fatal("invalid recipient should roll back freeze")
	}
	if _, loadErr := store.Load(context.Background(), bad.IdempotencyKey); !errors.Is(loadErr, sql.ErrNoRows) {
		t.Fatalf("invalid freeze left durable operation: %v", loadErr)
	}
}

func TestNotificationDeliveryStoreClaimCASHasSingleWinner(t *testing.T) {
	db := openNotificationDB(t, filepath.Join(t.TempDir(), "cas.db"))
	defer db.Close()
	store := NotificationDeliveryStore{DB: db}
	op := gateway.Operation{ExecutionID: "incident-cas", IdempotencyKey: "notify-cas"}
	if _, _, err := store.Freeze(context.Background(), op, "digest", []notificationRecipient{{TelegramUserID: 1, UserID: "admin", ChatID: 901}}); err != nil {
		t.Fatal(err)
	}

	const workers = 12
	var wg sync.WaitGroup
	wg.Add(workers)
	winners := make(chan bool, workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			delivery, claimed, err := store.ClaimPrepared(context.Background(), op.IdempotencyKey, 1)
			if err != nil {
				errs <- err
				return
			}
			if delivery.State != deliverySending {
				errs <- errors.New("claim did not observe sending state")
				return
			}
			winners <- claimed
		}()
	}
	wg.Wait()
	close(winners)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	count := 0
	for won := range winners {
		if won {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("CAS winners=%d want=1", count)
	}

	if err := store.MarkApplied(context.Background(), op.IdempotencyKey, 1, 4242); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background(), op.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Deliveries[0].State != deliveryApplied || loaded.Deliveries[0].ProviderMessageID != 4242 {
		t.Fatalf("applied delivery=%+v", loaded.Deliveries[0])
	}
	if err := store.MarkApplied(context.Background(), op.IdempotencyKey, 1, 4243); err == nil {
		t.Fatal("stale applied transition unexpectedly succeeded")
	}
	if err := store.MarkApplied(context.Background(), op.IdempotencyKey, 1, 0); err == nil {
		t.Fatal("zero provider message id accepted")
	}
}

func TestNotificationDeliveryStoreExplicitRetryAndAmbiguousTransitions(t *testing.T) {
	db := openNotificationDB(t, filepath.Join(t.TempDir(), "transitions.db"))
	defer db.Close()
	store := NotificationDeliveryStore{DB: db}

	prepare := func(key string, telegramUserID int64, chatID int64) gateway.Operation {
		op := gateway.Operation{ExecutionID: "incident-" + key, IdempotencyKey: key}
		if _, _, err := store.Freeze(context.Background(), op, "digest-"+key, []notificationRecipient{{TelegramUserID: telegramUserID, UserID: "admin", ChatID: chatID}}); err != nil {
			t.Fatal(err)
		}
		if _, claimed, err := store.ClaimPrepared(context.Background(), key, telegramUserID); err != nil || !claimed {
			t.Fatalf("claim key=%s claimed=%v err=%v", key, claimed, err)
		}
		return op
	}

	retry := prepare("notify-retry", 1, 1001)
	if err := store.ReleasePrepared(context.Background(), retry.IdempotencyKey, 1); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background(), retry.IdempotencyKey)
	if err != nil || loaded.Deliveries[0].State != deliveryPrepared {
		t.Fatalf("released state=%+v err=%v", loaded, err)
	}
	if _, claimed, err := store.ClaimPrepared(context.Background(), retry.IdempotencyKey, 1); err != nil || !claimed {
		t.Fatalf("reclaim claimed=%v err=%v", claimed, err)
	}

	ambiguous := prepare("notify-unknown", 2, 1002)
	if err := store.MarkAmbiguous(context.Background(), ambiguous.IdempotencyKey, 2); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.Load(context.Background(), ambiguous.IdempotencyKey)
	if err != nil || loaded.Deliveries[0].State != deliveryAmbiguous {
		t.Fatalf("ambiguous state=%+v err=%v", loaded, err)
	}
	if err := store.ReleasePrepared(context.Background(), ambiguous.IdempotencyKey, 2); err == nil {
		t.Fatal("ambiguous delivery became retryable")
	}
}

func TestNotificationDeliveryStoreInputValidation(t *testing.T) {
	db := openNotificationDB(t, filepath.Join(t.TempDir(), "validation.db"))
	defer db.Close()
	store := NotificationDeliveryStore{DB: db}

	if _, _, err := store.Freeze(context.Background(), gateway.Operation{}, "digest", []notificationRecipient{{TelegramUserID: 1, UserID: "admin", ChatID: 1}}); err == nil {
		t.Fatal("invalid gateway operation accepted")
	}
	if _, _, err := store.Freeze(context.Background(), gateway.Operation{ExecutionID: "incident", IdempotencyKey: "key"}, " ", []notificationRecipient{{TelegramUserID: 1, UserID: "admin", ChatID: 1}}); err == nil {
		t.Fatal("empty digest accepted")
	}
	if _, _, err := store.Freeze(context.Background(), gateway.Operation{ExecutionID: "incident", IdempotencyKey: "key"}, "digest", nil); !errors.Is(err, ErrNoNotificationRecipients) {
		t.Fatalf("empty recipients error=%v", err)
	}

	nilStore := NotificationDeliveryStore{}
	if _, err := nilStore.Load(context.Background(), "key"); err == nil {
		t.Fatal("nil db load accepted")
	}
	if _, _, err := nilStore.ClaimPrepared(context.Background(), "key", 1); err == nil {
		t.Fatal("nil db claim accepted")
	}
	if err := nilStore.MarkAmbiguous(context.Background(), "key", 1); err == nil {
		t.Fatal("nil db transition accepted")
	}
}
