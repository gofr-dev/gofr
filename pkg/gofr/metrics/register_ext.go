package metrics

import "context"

func (m *metricsManager) Shutdown(ctx context.Context) error {
	if m.flush != nil {
		return m.flush(ctx)
	}
	return nil
}
