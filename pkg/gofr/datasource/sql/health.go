package sql

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

type DBStats struct {
	MaxOpenConnections int `json:"maxOpenConnections"` // Maximum number of open connections to the database.

	// Pool Status
	OpenConnections int `json:"openConnections"` // The number of established connections both in use and idle.
	InUse           int `json:"inUse"`           // The number of connections currently in use.
	Idle            int `json:"idle"`            // The number of idle connections.

	// Counters
	WaitCount         int64         `json:"waitCount"`         // The total number of connections waited for.
	WaitDuration      time.Duration `json:"waitDuration"`      // The total time blocked waiting for a new connection.
	MaxIdleClosed     int64         `json:"maxIdleClosed"`     // The total number of connections closed due to SetMaxIdleConns.
	MaxIdleTimeClosed int64         `json:"maxIdleTimeClosed"` // The total number of connections closed due to SetConnMaxIdleTime.
	MaxLifetimeClosed int64         `json:"maxLifetimeClosed"` // The total number of connections closed due to SetConnMaxLifetime.
}

func (d *DB) HealthCheck() *datasource.Health {
	h := datasource.Health{
		Details: make(map[string]any),
	}

	h.Details["host"] = d.config.HostName + ":" + d.config.Port + "/" + d.config.Database

	if d.DB == nil {
		d.logger.Warn("HealthCheck: DB connection is nil")
		h.Status = datasource.StatusDown
		return &h
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ctx, span := d.tracer.Start(ctx, "DB.HealthCheck")
	defer span.End()

	d.logger.Debug("HealthCheck: pinging database...")

	if err := d.PingContext(ctx); err != nil {
		d.logger.Errorf("HealthCheck: DB ping failed, error: %v", err)
		h.Status = datasource.StatusDown
		return &h
	}

	d.logger.Debug("HealthCheck: ping successful. Collecting stats.")

	h.Status = datasource.StatusUp

	stats := d.Stats()
	h.Details["stats"] = DBStats{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxIdleTimeClosed:  stats.MaxIdleTimeClosed,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,
	}

	d.logger.Debugf("HealthCheck: stats collected: %+v", h.Details["stats"])

	return &h
}
