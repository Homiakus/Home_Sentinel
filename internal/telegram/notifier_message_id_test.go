package telegram

import (
	"context"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
)

func TestDurableNotifierProviderMessageIDLowerBound(t *testing.T) {
	for _, tc := range []struct {
		name      string
		messageID int64
		want      gateway.EffectState
	}{
		{name: "minimum valid", messageID: 1, want: gateway.EffectApplied},
		{name: "zero invalid", messageID: 0, want: gateway.EffectAmbiguous},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sender := &fakeNotificationSender{outcomes: map[int64][]notificationSendOutcome{
				1101: {{messageID: tc.messageID}},
			}}
			notifier, _, _, _, db := notificationFixture(t,
				[]Binding{{TelegramUserID: 1, UserID: "admin", ChatID: 1101}},
				map[string]auth.User{"admin": {ID: "admin", Role: auth.RoleAdmin}}, sender)
			defer db.Close()

			result, err := notifier.Notify(context.Background(), gateway.Operation{
				ExecutionID: "incident-message-id-" + tc.name,
				IdempotencyKey: "notify-message-id-" + tc.name,
			}, ownerNotification("provider id boundary"))
			if err != nil {
				t.Fatalf("Notify() error=%v", err)
			}
			if result.State != tc.want {
				t.Fatalf("Notify() state=%s want=%s", result.State, tc.want)
			}
			if sender.totalCalls() != 1 {
				t.Fatalf("provider calls=%d want=1", sender.totalCalls())
			}
		})
	}
}
