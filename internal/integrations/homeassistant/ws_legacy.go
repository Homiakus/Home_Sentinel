//go:build !go1.24

package homeassistant

import "context"

func newWSRuntime(context.Context, WSOptions, func(bool), func(StreamEvent)) (wsRuntime, error) {
	return nil, ErrWebSocketRuntimeUnsupported
}
