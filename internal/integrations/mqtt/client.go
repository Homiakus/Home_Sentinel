package mqtt

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

var ErrRuntimeUnsupported = errors.New("MQTT runtime requires Go 1.24 or newer")

type Message struct {
	Topic    string
	Payload  []byte
	QoS      byte
	Retained bool
}

type Subscription struct {
	Topic string
	QoS   byte
}

type Handler func(context.Context, Message)

type Options struct {
	URL                     string
	ClientID                string
	Username                string
	Password                []byte
	KeepAlive               time.Duration
	SessionExpiry           time.Duration
	ConnectTimeout          time.Duration
	Subscriptions           []Subscription
	Handler                 Handler
	OnConnectionStateChange func(bool)
}

type Client struct {
	runtime runtimeClient
	ready   atomic.Bool
}

type runtimeClient interface {
	Publish(context.Context, Message) error
	AwaitConnection(context.Context) error
	Close() error
}

func NewClient(ctx context.Context, opts Options) (*Client, error) {
	if ctx == nil {
		return nil, errors.New("MQTT context required")
	}
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	c := &Client{}
	state := func(up bool) {
		c.ready.Store(up)
		if opts.OnConnectionStateChange != nil {
			opts.OnConnectionStateChange(up)
		}
	}
	r, err := newRuntimeClient(ctx, opts, state)
	if err != nil {
		return nil, err
	}
	c.runtime = r
	return c, nil
}

func validateOptions(opts Options) error {
	if strings.TrimSpace(opts.URL) == "" {
		return errors.New("MQTT URL required")
	}
	u, err := url.Parse(opts.URL)
	if err != nil || u.Host == "" {
		return errors.New("MQTT URL invalid")
	}
	switch strings.ToLower(u.Scheme) {
	case "mqtt", "tls", "ws", "wss":
	default:
		return fmt.Errorf("unsupported MQTT URL scheme %q", u.Scheme)
	}
	if strings.TrimSpace(opts.ClientID) == "" {
		return errors.New("MQTT client id required")
	}
	if opts.KeepAlive <= 0 || opts.KeepAlive > 18*time.Hour {
		return errors.New("MQTT keepalive must be between 1s and 18h")
	}
	if opts.SessionExpiry < 0 || opts.SessionExpiry > time.Duration(^uint32(0))*time.Second {
		return errors.New("MQTT session expiry out of range")
	}
	if opts.ConnectTimeout <= 0 {
		return errors.New("MQTT connect timeout must be positive")
	}
	for _, sub := range opts.Subscriptions {
		if err := ValidateSubscribeTopic(sub.Topic); err != nil {
			return fmt.Errorf("subscription %q: %w", sub.Topic, err)
		}
		if sub.QoS > 2 {
			return fmt.Errorf("subscription %q: invalid qos %d", sub.Topic, sub.QoS)
		}
	}
	return nil
}

func (c *Client) Ready() bool { return c != nil && c.ready.Load() }

func (c *Client) AwaitConnection(ctx context.Context) error {
	if c == nil || c.runtime == nil {
		return errors.New("MQTT client unavailable")
	}
	return c.runtime.AwaitConnection(ctx)
}

func (c *Client) Publish(ctx context.Context, msg Message) error {
	if c == nil || c.runtime == nil {
		return errors.New("MQTT client unavailable")
	}
	if err := ValidatePublishTopic(msg.Topic); err != nil {
		return err
	}
	if msg.QoS > 2 {
		return fmt.Errorf("invalid MQTT qos %d", msg.QoS)
	}
	msg.Payload = append([]byte(nil), msg.Payload...)
	return c.runtime.Publish(ctx, msg)
}

func (c *Client) Close() error {
	if c == nil || c.runtime == nil {
		return nil
	}
	return c.runtime.Close()
}
