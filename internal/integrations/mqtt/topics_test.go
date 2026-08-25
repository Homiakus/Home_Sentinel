package mqtt

import "testing"

func TestTopicContract(t *testing.T) {
	got, err := IntercomCommand("door_front", "unlock")
	if err != nil || got != "sentinel/intercom/door_front/command/unlock" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	for _, bad := range []string{"sentinel/#", "/sentinel/x", "sentinel//x", "$SYS/x"} {
		if ValidatePublishTopic(bad) == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
