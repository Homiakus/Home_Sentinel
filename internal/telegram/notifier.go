package telegram

import (
	"cmp"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/authz"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	tgapi "github.com/Homiakus/Home_Sentinel/internal/integrations/telegram"
)

const (
	notificationDigestVersion = 1
	telegramTextLimit         = 4096
	maxClaimAttempts          = 3
)

var (
	ErrNoNotificationRecipients       = errors.New("telegram notifier: no authorized notification recipients")
	ErrNotificationConflict           = errors.New("telegram notifier: idempotency key reused with different semantics")
	ErrNotificationRecipientConflict  = errors.New("telegram notifier: conflicting eligible Telegram recipients")
	ErrUnsupportedNotificationMedia   = errors.New("telegram notifier: media notifications are not supported")
	ErrUnsupportedNotificationChannel = errors.New("telegram notifier: unsupported notification channel")
)

type notificationSender interface {
	SendMessage(context.Context, tgapi.SendMessageRequest) (tgapi.Message, error)
}

type notificationPairings interface {
	ListBindings(context.Context) ([]Binding, error)
}

type notificationDeliveryStore interface {
	Freeze(context.Context, gateway.Operation, string, []notificationRecipient) (notificationOperation, bool, error)
	Load(context.Context, string) (notificationOperation, error)
	ClaimPrepared(context.Context, string, int64) (notificationDelivery, bool, error)
	MarkApplied(context.Context, string, int64, int64) error
	MarkAmbiguous(context.Context, string, int64) error
	ReleasePrepared(context.Context, string, int64) error
}

// DurableNotifier is the production Telegram implementation of gateway.Notifier.
// Provider calls are fenced by durable per-recipient receipts. In particular,
// a process restart with a recipient left in sending state is never repaired by
// blind resend because Telegram SendMessage has no provider-side idempotency key.
type DurableNotifier struct {
	Sender     notificationSender
	Pairings   notificationPairings
	Users      UserLookup
	Deliveries notificationDeliveryStore
	Capability authz.Capability
}

type canonicalNotification struct {
	Version int    `json:"version"`
	Channel string `json:"channel"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Text    string `json:"-"`
}

func (n *DurableNotifier) Notify(ctx context.Context, op gateway.Operation, in gateway.Notification) (gateway.EffectResult, error) {
	if n == nil || n.Sender == nil || n.Pairings == nil || n.Users == nil || n.Deliveries == nil {
		return gateway.EffectResult{}, errors.New("telegram notifier: dependencies unavailable")
	}
	if err := op.Validate(); err != nil {
		return gateway.EffectResult{}, err
	}
	canonical, digest, err := canonicalizeNotification(in)
	if err != nil {
		return gateway.EffectResult{}, err
	}

	frozen, _, err := n.loadOrFreeze(ctx, op, digest)
	if err != nil {
		return gateway.EffectResult{}, err
	}
	if err := validateFrozenNotification(frozen, op, digest); err != nil {
		return gateway.EffectResult{}, err
	}
	if result, done := summarizeFrozenNotification(frozen, false); done {
		return result, nil
	}

	sentAny := false
	for _, durable := range frozen.Deliveries {
		if durable.State == deliveryApplied {
			continue
		}
		if durable.State == deliverySending || durable.State == deliveryAmbiguous {
			return ambiguousNotificationResult(frozen), nil
		}
		if durable.State != deliveryPrepared {
			return gateway.EffectResult{}, fmt.Errorf("telegram notifier: unsupported delivery state %q", durable.State)
		}

		claimedDelivery, claimed, err := n.claimPrepared(ctx, frozen.IdempotencyKey, durable.Recipient.TelegramUserID)
		if err != nil {
			return gateway.EffectResult{}, err
		}
		if !claimed {
			switch claimedDelivery.State {
			case deliveryApplied:
				continue
			case deliverySending, deliveryAmbiguous:
				return ambiguousNotificationResult(frozen), nil
			default:
				return gateway.EffectResult{}, fmt.Errorf("telegram notifier: delivery claim did not converge from state %q", claimedDelivery.State)
			}
		}

		message, sendErr := n.Sender.SendMessage(ctx, tgapi.SendMessageRequest{
			ChatID:                claimedDelivery.Recipient.ChatID,
			Text:                  canonical.Text,
			DisableWebPagePreview: true,
		})
		if sendErr != nil {
			var apiErr *tgapi.APIError
			if errors.As(sendErr, &apiErr) {
				// A Telegram Bot API rejection is an explicit provider response: the
				// message was not accepted. It is safe to make this recipient retryable
				// only if the durable sending->prepared transition succeeds.
				if releaseErr := n.Deliveries.ReleasePrepared(ctx, frozen.IdempotencyKey, claimedDelivery.Recipient.TelegramUserID); releaseErr != nil {
					return ambiguousNotificationResult(frozen), nil
				}
				return gateway.EffectResult{}, fmt.Errorf("telegram notifier: provider rejected delivery: %w", sendErr)
			}

			// Transport, context and malformed-response failures may happen after
			// Telegram accepted the request. Mark ambiguity when possible; if the
			// persistence write itself fails, the durable sending state is already
			// sufficient to prohibit blind resend after retry/restart.
			_ = n.Deliveries.MarkAmbiguous(ctx, frozen.IdempotencyKey, claimedDelivery.Recipient.TelegramUserID)
			return ambiguousNotificationResult(frozen), nil
		}
		if message.MessageID < 1 {
			_ = n.Deliveries.MarkAmbiguous(ctx, frozen.IdempotencyKey, claimedDelivery.Recipient.TelegramUserID)
			return ambiguousNotificationResult(frozen), nil
		}
		if err := n.Deliveries.MarkApplied(ctx, frozen.IdempotencyKey, claimedDelivery.Recipient.TelegramUserID, message.MessageID); err != nil {
			_ = n.Deliveries.MarkAmbiguous(ctx, frozen.IdempotencyKey, claimedDelivery.Recipient.TelegramUserID)
			return ambiguousNotificationResult(frozen), nil
		}
		sentAny = true
	}

	final, err := n.Deliveries.Load(ctx, frozen.IdempotencyKey)
	if err != nil {
		return gateway.EffectResult{}, err
	}
	if err := validateFrozenNotification(final, op, digest); err != nil {
		return gateway.EffectResult{}, err
	}
	if result, done := summarizeFrozenNotification(final, sentAny); done {
		return result, nil
	}
	return gateway.EffectResult{}, errors.New("telegram notifier: delivery operation did not converge")
}

func (n *DurableNotifier) loadOrFreeze(ctx context.Context, op gateway.Operation, digest string) (notificationOperation, bool, error) {
	frozen, err := n.Deliveries.Load(ctx, strings.TrimSpace(op.IdempotencyKey))
	if err == nil {
		return frozen, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return notificationOperation{}, false, err
	}
	recipients, err := n.selectRecipients(ctx)
	if err != nil {
		return notificationOperation{}, false, err
	}
	frozen, created, err := n.Deliveries.Freeze(ctx, op, digest, recipients)
	if err != nil {
		return notificationOperation{}, false, err
	}
	if !created && !sameNotificationRecipients(frozen.Deliveries, recipients) {
		return notificationOperation{}, false, errors.Join(
			ErrNotificationConflict,
			ErrNotificationRecipientConflict,
			errors.New("telegram notifier: concurrent initial recipient freeze disagreed"),
		)
	}
	return frozen, created, nil
}

func (n *DurableNotifier) selectRecipients(ctx context.Context) ([]notificationRecipient, error) {
	bindings, err := n.Pairings.ListBindings(ctx)
	if err != nil {
		return nil, fmt.Errorf("telegram notifier: list bindings: %w", err)
	}
	capability := n.Capability
	if capability == "" {
		capability = authz.AcknowledgeIncident
	}
	seenChat := make(map[int64]struct{}, len(bindings))
	recipients := make([]notificationRecipient, 0, len(bindings))
	for _, binding := range bindings {
		if binding.TelegramUserID <= 0 || binding.ChatID == 0 || strings.TrimSpace(binding.UserID) == "" {
			return nil, errors.New("telegram notifier: malformed active binding")
		}
		user, err := n.Users.GetByID(ctx, binding.UserID)
		if err != nil {
			return nil, fmt.Errorf("telegram notifier: resolve bound user %q: %w", binding.UserID, err)
		}
		if user.Disabled || !authz.Allowed(user.Role, capability) {
			continue
		}
		if _, exists := seenChat[binding.ChatID]; exists {
			return nil, ErrNotificationRecipientConflict
		}
		seenChat[binding.ChatID] = struct{}{}
		recipients = append(recipients, notificationRecipient{
			TelegramUserID: binding.TelegramUserID,
			UserID:         strings.TrimSpace(binding.UserID),
			ChatID:         binding.ChatID,
		})
	}
	if len(recipients) == 0 {
		return nil, ErrNoNotificationRecipients
	}
	slices.SortFunc(recipients, func(a, b notificationRecipient) int {
		return cmp.Compare(a.TelegramUserID, b.TelegramUserID)
	})
	return recipients, nil
}

func (n *DurableNotifier) claimPrepared(ctx context.Context, idempotencyKey string, telegramUserID int64) (notificationDelivery, bool, error) {
	var last notificationDelivery
	for range maxClaimAttempts {
		delivery, claimed, err := n.Deliveries.ClaimPrepared(ctx, idempotencyKey, telegramUserID)
		if err != nil {
			return notificationDelivery{}, false, err
		}
		last = delivery
		if claimed {
			return delivery, true, nil
		}
		if delivery.State != deliveryPrepared {
			return delivery, false, nil
		}
	}
	return last, false, errors.New("telegram notifier: delivery claim contention did not converge")
}

func canonicalizeNotification(in gateway.Notification) (canonicalNotification, string, error) {
	if len(in.Media) != 0 {
		return canonicalNotification{}, "", ErrUnsupportedNotificationMedia
	}
	channel := strings.ToLower(strings.TrimSpace(in.Channel))
	if channel != "owner" {
		return canonicalNotification{}, "", ErrUnsupportedNotificationChannel
	}
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	if title == "" && body == "" {
		return canonicalNotification{}, "", errors.New("telegram notifier: notification text is required")
	}
	text := title
	if text != "" && body != "" {
		text += "\n\n" + body
	} else if text == "" {
		text = body
	}
	if utf8.RuneCountInString(text) > telegramTextLimit {
		return canonicalNotification{}, "", fmt.Errorf("telegram notifier: notification exceeds %d characters", telegramTextLimit)
	}
	canonical := canonicalNotification{
		Version: notificationDigestVersion,
		Channel: channel,
		Title:   title,
		Body:    body,
		Text:    text,
	}
	raw, err := json.Marshal(canonical)
	if err != nil {
		return canonicalNotification{}, "", err
	}
	sum := sha256.Sum256(raw)
	return canonical, hex.EncodeToString(sum[:]), nil
}

func validateFrozenNotification(frozen notificationOperation, op gateway.Operation, digest string) error {
	if frozen.ExecutionID != strings.TrimSpace(op.ExecutionID) || frozen.SemanticDigest != digest {
		return errors.Join(
			ErrNotificationConflict,
			fmt.Errorf("telegram notifier: frozen operation %q does not match requested execution/notification", strings.TrimSpace(op.IdempotencyKey)),
		)
	}
	return nil
}

func sameNotificationRecipients(deliveries []notificationDelivery, recipients []notificationRecipient) bool {
	if len(deliveries) != len(recipients) {
		return false
	}
	for i := range recipients {
		if deliveries[i].Recipient != recipients[i] {
			return false
		}
	}
	return true
}

func summarizeFrozenNotification(frozen notificationOperation, appliedThisCall bool) (gateway.EffectResult, bool) {
	if len(frozen.Deliveries) == 0 {
		return gateway.EffectResult{}, false
	}
	allApplied := true
	for _, delivery := range frozen.Deliveries {
		switch delivery.State {
		case deliverySending, deliveryAmbiguous:
			return ambiguousNotificationResult(frozen), true
		case deliveryApplied:
		default:
			allApplied = false
		}
	}
	if !allApplied {
		return gateway.EffectResult{}, false
	}
	state := gateway.EffectAlreadyApplied
	if appliedThisCall {
		state = gateway.EffectApplied
	}
	providerID := ""
	if len(frozen.Deliveries) == 1 && frozen.Deliveries[0].ProviderMessageID > 0 {
		providerID = strconv.FormatInt(frozen.Deliveries[0].ProviderMessageID, 10)
	}
	return gateway.EffectResult{
		State:       state,
		ProviderID:  providerID,
		Description: fmt.Sprintf("telegram notification confirmed for %d recipient(s)", len(frozen.Deliveries)),
	}, true
}

func ambiguousNotificationResult(frozen notificationOperation) gateway.EffectResult {
	return gateway.EffectResult{
		State:       gateway.EffectAmbiguous,
		Description: fmt.Sprintf("telegram notification outcome is ambiguous for operation %q", frozen.IdempotencyKey),
	}
}

var _ gateway.Notifier = (*DurableNotifier)(nil)
var _ notificationSender = (*tgapi.Client)(nil)
var _ UserLookup = (*auth.UserStore)(nil)
