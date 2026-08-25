//go:build !go1.24

package mqtt

import "context"

func newRuntimeClient(context.Context, Options, func(bool)) (runtimeClient, error) {
	return nil, ErrRuntimeUnsupported
}
