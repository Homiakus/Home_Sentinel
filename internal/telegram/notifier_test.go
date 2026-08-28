package telegram

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/domain/artifact"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	tgapi "github.com/Homiakus/Home_Sentinel/internal/integrations/telegram"
)

type notificationSendOutcome struct {
	messageID int64
	err       error
}

type fakeNotificationSender struct {
	mu       sync.Mutex
	calls    []tgapi.SendMessageRequest
	byChat   map[int64]int
	outcomes map[int64][]notificationSendOutcome
	started  chan struct{}
	block    chan struct{}
	once     sync.Once
}

func (f *fakeNotificationSender) SendMessage(_ context.Context, in tgapi.SendMessageRequest) (tgapi.Message, error) {
	f.mu.Lock()
	f.calls = append(f.calls, in)
	if f.byChat == nil {
		f.byChat = map[int64]int{}
	}
	index := f.byChat[in.ChatID]
	f.byChat[in.ChatID] = index + 1
	var outcome notificationSendOutcome
	if sequence := f.outcomes[in.ChatID]; index < len(sequence) {
		outcome = sequence[index]
	} else {
		outcome.messageID = int64(1000 + len(f.calls))
	}
	started := f.started
	block := f.block
	f.mu.Unlock()

	if started != nil {
		f.once.Do(func() { close(started) })
	}
	if block != nil {
		<-block
	}
	if outcome.err != nil {
		return tgapi.Message{}, outcome.err
	}
	return tgapi.Message{MessageID: outcome.messageID}, nil
}

func (f *fakeNotificationSender) callCount(chatID int64) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byChat[chatID]
}

func (f *fakeNotificationSender) totalCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeNotificationSender) firstCall() tgapi.SendMessageRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return tgapi.SendMessageRequest{}
	}
	return f.calls[0]
}

type fakeNotificationPairings struct {
	mu       sync.Mutex
	bindings []Binding
	err      error
	calls    int
}

func (f *fakeNotificationPairings) ListBindings(context.Context) ([]Binding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return append([]Binding(nil), f.bindings...), nil
}

func (f *fakeNotificationPairings) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type fakeNotificationUsers struct {
	mu    sync.Mutex
	users map[string]auth.User
	errs  map[string]error
	calls int
}

func (f *fakeNotificationUsers) GetByID(_ context.Context, id string) (auth.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if err := f.errs[id]; err != nil {
		return auth.User{}, err
	}
	user, ok := f.users[id]
	if !ok {
		return auth.User{}, errors.New("user not found")
	}
	return user, nil
}

func (f *fakeNotificationUsers) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func openNotificationDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := database.Open(context.Background(), database.Options{
		Path:        path,
		BusyTimeout: 3 * time.Second,
		MaxOpen:     8,
		MaxIdle:     4,
	})
	if err != nil {
		t.Fatalf("open notification db: %v", err)
	}
	if err := database.Migrate(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate notification db: %v", err)
	}
	return db
}

func notificationFixture(t *testing.T, bindings []Binding, users map[string]auth.User, sender *fakeNotificationSender) (*DurableNotifier, NotificationDeliveryStore, *fakeNotificationPairings, *fakeNotificationUsers, *sql.DB) {
	t.Helper()
	db := openNotificationDB(t, filepath.Join(t.TempDir(), "notification.db"))
	store := NotificationDeliveryStore{DB: db}
	pairings := &fakeNotificationPairings{bindings: bindings}
	lookup := &fakeNotificationUsers{users: users, errs: map[string]error{}}
	if sender == nil {
		sender = &fakeNotificationSender{}
	}
	notifier := &DurableNotifier{Sender: sender, Pairings: pairings, Users: lookup, Deliveries: store}
	return notifier, store, pairings, lookup, db
}

func ownerNotification(body string) gateway.Notification {
	return gateway.Notification{Channel: "owner", Title: "Home Sentinel incident", Body: body}
}

func TestDurableNotifierSuccessRetryUsesFrozenRecipients(t *testing.T) {
	sender := &fakeNotificationSender{}
	notifier, store, pairings, users, db := notificationFixture(t,
		[]Binding{
			{TelegramUserID: 1, UserID: "operator", ChatID: 101},
			{TelegramUserID: 2, UserID: "viewer", ChatID: 102},
			{TelegramUserID: 3, UserID: "disabled", ChatID: 103},
		},
		map[string]auth.User{
			"operator": {ID: "operator", Role: auth.RoleOperator},
			"viewer":   {ID: "viewer", Role: auth.RoleViewer},
			"disabled": {ID: "disabled", Role: auth.RoleAdmin, Disabled: true},
		}, sender)
	defer db.Close()

	op := gateway.Operation{ExecutionID: "incident-1", IdempotencyKey: "notify-1"}
	result, err := notifier.Notify(context.Background(), op, ownerNotification("doorbell detected"))
	if err != nil {
		t.Fatalf("first notify: %v", err)
	}
	if result.State != gateway.EffectApplied || result.ProviderID == "" {
		t.Fatalf("first result=%+v", result)
	}
	if sender.totalCalls() != 1 || sender.callCount(101) != 1 {
		t.Fatalf("provider calls=%d operator=%d", sender.totalCalls(), sender.callCount(101))
	}
	request := sender.firstCall()
	if request.ChatID != 101 || request.Text != "Home Sentinel incident\n\ndoorbell detected" || !request.DisableWebPagePreview {
		t.Fatalf("provider request=%+v", request)
	}
	firstPairingCalls := pairings.callCount()
	firstUserCalls := users.callCount()

	// A retry must use the durable frozen recipient set. Deliberately make live
	// authorization sources unavailable to prove they are not consulted again.
	pairings.mu.Lock()
	pairings.err = errors.New("pairing store unavailable after freeze")
	pairings.mu.Unlock()
	users.mu.Lock()
	users.errs["operator"] = errors.New("user store unavailable after freeze")
	users.mu.Unlock()

	result, err = notifier.Notify(context.Background(), op, ownerNotification("doorbell detected"))
	if err != nil {
		t.Fatalf("retry notify: %v", err)
	}
	if result.State != gateway.EffectAlreadyApplied {
		t.Fatalf("retry result=%+v", result)
	}
	if sender.totalCalls() != 1 {
		t.Fatalf("exact retry resent provider call: %d", sender.totalCalls())
	}
	if pairings.callCount() != firstPairingCalls || users.callCount() != firstUserCalls {
		t.Fatalf("retry re-evaluated recipients: pairings %d->%d users %d->%d", firstPairingCalls, pairings.callCount(), firstUserCalls, users.callCount())
	}

	frozen, err := store.Load(context.Background(), op.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(frozen.Deliveries) != 1 || frozen.Deliveries[0].Recipient.UserID != "operator" || frozen.Deliveries[0].State != deliveryApplied {
		t.Fatalf("frozen operation=%+v", frozen)
	}
}

func TestDurableNotifierSemanticReuseFailsClosed(t *testing.T) {
	sender := &fakeNotificationSender{}
	notifier, _, _, _, db := notificationFixture(t,
		[]Binding{{TelegramUserID: 1, UserID: "admin", ChatID: 201}},
		map[string]auth.User{"admin": {ID: "admin", Role: auth.RoleAdmin}}, sender)
	defer db.Close()

	op := gateway.Operation{ExecutionID: "incident-1", IdempotencyKey: "notify-conflict"}
	if _, err := notifier.Notify(context.Background(), op, ownerNotification("original")); err != nil {
		t.Fatal(err)
	}
	initialCalls := sender.totalCalls()

	if _, err := notifier.Notify(context.Background(), op, ownerNotification("changed")); !errors.Is(err, ErrNotificationConflict) {
		t.Fatalf("changed body error=%v", err)
	}
	changedExecution := op
	changedExecution.ExecutionID = "incident-2"
	if _, err := notifier.Notify(context.Background(), changedExecution, ownerNotification("original")); !errors.Is(err, ErrNotificationConflict) {
		t.Fatalf("changed execution error=%v", err)
	}
	if sender.totalCalls() != initialCalls {
		t.Fatalf("conflicting replay reached provider: %d->%d", initialCalls, sender.totalCalls())
	}
}

func TestDurableNotifierPartialFanoutRetriesOnlyPreparedRecipient(t *testing.T) {
	sender := &fakeNotificationSender{outcomes: map[int64][]notificationSendOutcome{
		301: {{messageID: 3011}},
		302: {{err: &tgapi.APIError{Code: 429, Description: "retry later", RetryAfter: 1}}, {messageID: 3022}},
	}}
	notifier, store, _, _, db := notificationFixture(t,
		[]Binding{
			{TelegramUserID: 1, UserID: "operator", ChatID: 301},
			{TelegramUserID: 2, UserID: "admin", ChatID: 302},
		},
		map[string]auth.User{
			"operator": {ID: "operator", Role: auth.RoleOperator},
			"admin":    {ID: "admin", Role: auth.RoleAdmin},
		}, sender)
	defer db.Close()

	op := gateway.Operation{ExecutionID: "incident-partial", IdempotencyKey: "notify-partial"}
	if _, err := notifier.Notify(context.Background(), op, ownerNotification("partial fanout")); err == nil {
		t.Fatal("provider rejection should be retryable error")
	}
	frozen, err := store.Load(context.Background(), op.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if got := []notificationDeliveryState{frozen.Deliveries[0].State, frozen.Deliveries[1].State}; got[0] != deliveryApplied || got[1] != deliveryPrepared {
		t.Fatalf("partial states=%v", got)
	}

	result, err := notifier.Notify(context.Background(), op, ownerNotification("partial fanout"))
	if err != nil {
		t.Fatalf("retry partial fanout: %v", err)
	}
	if result.State != gateway.EffectApplied {
		t.Fatalf("retry result=%+v", result)
	}
	if sender.callCount(301) != 1 || sender.callCount(302) != 2 {
		t.Fatalf("partial retry counts chat301=%d chat302=%d", sender.callCount(301), sender.callCount(302))
	}

	result, err = notifier.Notify(context.Background(), op, ownerNotification("partial fanout"))
	if err != nil || result.State != gateway.EffectAlreadyApplied {
		t.Fatalf("final replay result=%+v err=%v", result, err)
	}
	if sender.callCount(301) != 1 || sender.callCount(302) != 2 {
		t.Fatal("confirmed recipients were resent on final replay")
	}
}

func TestDurableNotifierUnknownProviderOutcomeBecomesAmbiguous(t *testing.T) {
	sender := &fakeNotificationSender{outcomes: map[int64][]notificationSendOutcome{
		401: {{err: errors.New("transport outcome unknown")}},
	}}
	notifier, store, _, _, db := notificationFixture(t,
		[]Binding{{TelegramUserID: 1, UserID: "operator", ChatID: 401}},
		map[string]auth.User{"operator": {ID: "operator", Role: auth.RoleOperator}}, sender)
	defer db.Close()

	op := gateway.Operation{ExecutionID: "incident-ambiguous", IdempotencyKey: "notify-ambiguous"}
	result, err := notifier.Notify(context.Background(), op, ownerNotification("unknown provider outcome"))
	if err != nil || result.State != gateway.EffectAmbiguous {
		t.Fatalf("first ambiguous result=%+v err=%v", result, err)
	}
	frozen, err := store.Load(context.Background(), op.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.Deliveries[0].State != deliveryAmbiguous {
		t.Fatalf("durable state=%s", frozen.Deliveries[0].State)
	}

	result, err = notifier.Notify(context.Background(), op, ownerNotification("unknown provider outcome"))
	if err != nil || result.State != gateway.EffectAmbiguous {
		t.Fatalf("replay ambiguous result=%+v err=%v", result, err)
	}
	if sender.totalCalls() != 1 {
		t.Fatalf("ambiguous replay resent provider call: %d", sender.totalCalls())
	}
}

func TestDurableNotifierCrashReopenSendingNeverBlindResends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")
	db := openNotificationDB(t, path)
	store := NotificationDeliveryStore{DB: db}
	op := gateway.Operation{ExecutionID: "incident-crash", IdempotencyKey: "notify-crash"}
	in := ownerNotification("crash window")
	_, digest, err := canonicalizeNotification(in)
	if err != nil {
		t.Fatal(err)
	}
	frozen, created, err := store.Freeze(context.Background(), op, digest, []notificationRecipient{{TelegramUserID: 1, UserID: "admin", ChatID: 501}})
	if err != nil || !created {
		t.Fatalf("freeze created=%v err=%v", created, err)
	}
	claimed, won, err := store.ClaimPrepared(context.Background(), frozen.IdempotencyKey, 1)
	if err != nil || !won || claimed.State != deliverySending {
		t.Fatalf("claim=%+v won=%v err=%v", claimed, won, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openNotificationDB(t, path)
	defer reopened.Close()
	sender := &fakeNotificationSender{}
	pairings := &fakeNotificationPairings{err: errors.New("must not evaluate live pairings after reopen")}
	users := &fakeNotificationUsers{users: map[string]auth.User{}, errs: map[string]error{}}
	notifier := &DurableNotifier{
		Sender: sender, Pairings: pairings, Users: users,
		Deliveries: NotificationDeliveryStore{DB: reopened},
	}
	result, err := notifier.Notify(context.Background(), op, in)
	if err != nil || result.State != gateway.EffectAmbiguous {
		t.Fatalf("reopen result=%+v err=%v", result, err)
	}
	if sender.totalCalls() != 0 || pairings.callCount() != 0 || users.callCount() != 0 {
		t.Fatalf("reopen performed unsafe work: sends=%d pairings=%d users=%d", sender.totalCalls(), pairings.callCount(), users.callCount())
	}
}

func TestDurableNotifierRecipientSelectionFailsClosed(t *testing.T) {
	tests := []struct {
		name     string
		bindings []Binding
		users    map[string]auth.User
		userErrs map[string]error
		wantErr  error
	}{
		{
			name:     "no authorized recipients",
			bindings: []Binding{{TelegramUserID: 1, UserID: "viewer", ChatID: 601}},
			users:    map[string]auth.User{"viewer": {ID: "viewer", Role: auth.RoleViewer}},
			wantErr:  ErrNoNotificationRecipients,
		},
		{
			name:     "missing bound user",
			bindings: []Binding{{TelegramUserID: 1, UserID: "missing", ChatID: 602}},
			users:    map[string]auth.User{},
			userErrs: map[string]error{"missing": errors.New("lookup failed")},
		},
		{
			name: "duplicate eligible chat",
			bindings: []Binding{
				{TelegramUserID: 1, UserID: "operator", ChatID: 603},
				{TelegramUserID: 2, UserID: "admin", ChatID: 603},
			},
			users: map[string]auth.User{
				"operator": {ID: "operator", Role: auth.RoleOperator},
				"admin":    {ID: "admin", Role: auth.RoleAdmin},
			},
			wantErr: ErrNotificationRecipientConflict,
		},
		{
			name:     "malformed active binding",
			bindings: []Binding{{TelegramUserID: 0, UserID: "operator", ChatID: 604}},
			users:    map[string]auth.User{"operator": {ID: "operator", Role: auth.RoleOperator}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sender := &fakeNotificationSender{}
			notifier, _, pairings, users, db := notificationFixture(t, tc.bindings, tc.users, sender)
			defer db.Close()
			if tc.userErrs != nil {
				users.errs = tc.userErrs
			}
			_, err := notifier.Notify(context.Background(), gateway.Operation{ExecutionID: "incident", IdempotencyKey: "key-" + strings.ReplaceAll(tc.name, " ", "-")}, ownerNotification("selection"))
			if err == nil {
				t.Fatal("expected selection failure")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v want %v", err, tc.wantErr)
			}
			if sender.totalCalls() != 0 || pairings.callCount() != 1 {
				t.Fatalf("unsafe selection side effect sends=%d pairingCalls=%d", sender.totalCalls(), pairings.callCount())
			}
		})
	}
}

func TestDurableNotifierCanonicalValidation(t *testing.T) {
	base := gateway.Notification{Channel: " OWNER ", Title: " Alert ", Body: " Body "}
	canonical, digest, err := canonicalizeNotification(base)
	if err != nil {
		t.Fatal(err)
	}
	if canonical.Channel != "owner" || canonical.Title != "Alert" || canonical.Body != "Body" || canonical.Text != "Alert\n\nBody" || digest == "" {
		t.Fatalf("canonical=%+v digest=%q", canonical, digest)
	}
	_, digest2, err := canonicalizeNotification(gateway.Notification{Channel: "owner", Title: "Alert", Body: "Body"})
	if err != nil || digest2 != digest {
		t.Fatalf("normalization digest changed: %q vs %q err=%v", digest, digest2, err)
	}
	if _, _, err := canonicalizeNotification(gateway.Notification{Channel: "owner", Media: []artifact.Ref{{URI: "file:test"}}, Body: "x"}); !errors.Is(err, ErrUnsupportedNotificationMedia) {
		t.Fatalf("media error=%v", err)
	}
	if _, _, err := canonicalizeNotification(gateway.Notification{Channel: "email", Body: "x"}); !errors.Is(err, ErrUnsupportedNotificationChannel) {
		t.Fatalf("channel error=%v", err)
	}
	if _, _, err := canonicalizeNotification(gateway.Notification{Channel: "owner"}); err == nil {
		t.Fatal("empty notification accepted")
	}
	if _, _, err := canonicalizeNotification(gateway.Notification{Channel: "owner", Body: strings.Repeat("я", telegramTextLimit+1)}); err == nil {
		t.Fatal("oversized notification accepted")
	}
}

func TestDurableNotifierConcurrentDuplicateAtMostOneProviderSend(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	sender := &fakeNotificationSender{started: started, block: release}
	notifier, _, _, _, db := notificationFixture(t,
		[]Binding{{TelegramUserID: 1, UserID: "admin", ChatID: 701}},
		map[string]auth.User{"admin": {ID: "admin", Role: auth.RoleAdmin}}, sender)
	defer db.Close()

	op := gateway.Operation{ExecutionID: "incident-concurrent", IdempotencyKey: "notify-concurrent"}
	in := ownerNotification("concurrent")
	type outcome struct {
		result gateway.EffectResult
		err    error
	}
	results := make(chan outcome, 2)
	go func() {
		result, err := notifier.Notify(context.Background(), op, in)
		results <- outcome{result: result, err: err}
	}()
	<-started
	go func() {
		result, err := notifier.Notify(context.Background(), op, in)
		results <- outcome{result: result, err: err}
	}()

	// The first provider call is blocked, so the first result available here
	// must come from the duplicate caller observing durable sending state.
	var second outcome
	select {
	case second = <-results:
	case <-time.After(time.Second):
		t.Fatal("duplicate caller did not converge while provider call was fenced")
	}
	if second.err != nil || second.result.State != gateway.EffectAmbiguous {
		t.Fatalf("duplicate caller result=%+v err=%v", second.result, second.err)
	}
	close(release)
	first := <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("concurrent errors: first=%v second=%v", first.err, second.err)
	}
	if sender.totalCalls() != 1 {
		t.Fatalf("concurrent duplicate provider sends=%d", sender.totalCalls())
	}
	states := map[gateway.EffectState]int{first.result.State: 1}
	states[second.result.State]++
	if states[gateway.EffectApplied] != 1 || states[gateway.EffectAmbiguous]+states[gateway.EffectAlreadyApplied] != 1 {
		t.Fatalf("concurrent states first=%s second=%s", first.result.State, second.result.State)
	}

	final, err := notifier.Notify(context.Background(), op, in)
	if err != nil || final.State != gateway.EffectAlreadyApplied || sender.totalCalls() != 1 {
		t.Fatalf("post-concurrency replay result=%+v err=%v sends=%d", final, err, sender.totalCalls())
	}
}
