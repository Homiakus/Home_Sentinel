package mqtt

import (
	"context"
	"errors"
	"testing"
	"time"
)

func validOptions() Options {
	return Options{
		URL:            "mqtt://127.0.0.1:1883",
		ClientID:       "home-sentinel-test",
		KeepAlive:      30 * time.Second,
		SessionExpiry:  10 * time.Minute,
		ConnectTimeout: 5 * time.Second,
		Subscriptions:  []Subscription{{Topic: FrigateReviews, QoS: 1}},
	}
}

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Options)
	}{
		{"missing URL", func(o *Options) { o.URL = "" }},
		{"bad scheme", func(o *Options) { o.URL = "http://broker:1883" }},
		{"missing client id", func(o *Options) { o.ClientID = "" }},
		{"bad qos", func(o *Options) { o.Subscriptions[0].QoS = 3 }},
		{"bad filter", func(o *Options) { o.Subscriptions[0].Topic = "frigate/#/oops" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := validOptions()
			tt.edit(&o)
			if err := validateOptions(o); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateSubscribeTopicWildcards(t *testing.T) {
	for _, topic := range []string{"frigate/#", "sentinel/intercom/+/state/door"} {
		if err := ValidateSubscribeTopic(topic); err != nil {
			t.Fatalf("%s: %v", topic, err)
		}
	}
	for _, topic := range []string{"frigate/#/oops", "frigate/review+", "$SYS/#", "a//b"} {
		if err := ValidateSubscribeTopic(topic); err == nil {
			t.Fatalf("expected %s to be rejected", topic)
		}
	}
}

func TestLegacyRuntimeIsExplicit(t *testing.T) {
	if err := validateOptions(validOptions()); err != nil {
		t.Fatal(err)
	}
	c, err := NewClient(context.Background(), validOptions())
	if err != nil && !errors.Is(err, ErrRuntimeUnsupported) {
		t.Fatalf("unexpected runtime error: %v", err)
	}
	if c != nil {
		_ = c.Close()
	}
}
