//go:build go1.24

package mqtt

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

type pahoRuntime struct {
	manager *autopaho.ConnectionManager
	cancel  context.CancelFunc
	done    <-chan struct{}
	once    sync.Once
}

func newRuntimeClient(parent context.Context, opts Options, onState func(bool)) (runtimeClient, error) {
	u, err := url.Parse(opts.URL)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)

	cfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{u},
		KeepAlive:                     uint16(opts.KeepAlive / time.Second),
		CleanStartOnInitialConnection: false,
		SessionExpiryInterval:         uint32(opts.SessionExpiry / time.Second),
		ConnectTimeout:                opts.ConnectTimeout,
		ConnectUsername:               opts.Username,
		ConnectPassword:               append([]byte(nil), opts.Password...),
		ClientConfig: paho.ClientConfig{
			ClientID: opts.ClientID,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					if opts.Handler != nil && pr.Packet != nil {
						opts.Handler(ctx, Message{
							Topic:    pr.Packet.Topic,
							Payload:  append([]byte(nil), pr.Packet.Payload...),
							QoS:      pr.Packet.QoS,
							Retained: pr.Packet.Retain,
						})
					}
					return true, nil
				},
			},
		},
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *paho.Connack) {
			onState(true)
			if len(opts.Subscriptions) == 0 {
				return
			}
			subs := make([]paho.SubscribeOptions, 0, len(opts.Subscriptions))
			for _, sub := range opts.Subscriptions {
				subs = append(subs, paho.SubscribeOptions{Topic: sub.Topic, QoS: sub.QoS})
			}
			subCtx, subCancel := context.WithTimeout(ctx, opts.ConnectTimeout)
			defer subCancel()
			_, _ = cm.Subscribe(subCtx, &paho.Subscribe{Subscriptions: subs})
		},
		OnConnectError:   func(error) { onState(false) },
		OnConnectionDown: func() bool { onState(false); return true },
	}
	cm, err := autopaho.NewConnection(ctx, cfg)
	if err != nil {
		cancel()
		return nil, err
	}
	return &pahoRuntime{manager: cm, cancel: cancel, done: cm.Done()}, nil
}

func (p *pahoRuntime) Publish(ctx context.Context, msg Message) error {
	if p == nil || p.manager == nil {
		return errors.New("MQTT runtime unavailable")
	}
	_, err := p.manager.Publish(ctx, &paho.Publish{QoS: msg.QoS, Topic: msg.Topic, Payload: msg.Payload, Retain: msg.Retained})
	return err
}

func (p *pahoRuntime) AwaitConnection(ctx context.Context) error {
	if p == nil || p.manager == nil {
		return errors.New("MQTT runtime unavailable")
	}
	return p.manager.AwaitConnection(ctx)
}

func (p *pahoRuntime) Close() error {
	if p == nil {
		return nil
	}
	p.once.Do(func() { p.cancel() })
	if p.done != nil {
		<-p.done
	}
	return nil
}
