package telemetry

import "sync/atomic"

type Metrics struct {
	HTTPRequests atomic.Uint64
	HTTPErrors   atomic.Uint64
}

func (m *Metrics) ObserveHTTP(status int) {
	if m == nil {
		return
	}
	m.HTTPRequests.Add(1)
	if status >= 500 {
		m.HTTPErrors.Add(1)
	}
}
