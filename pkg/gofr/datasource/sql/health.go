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
		h.Status = datasource.StatusDown

		return &h
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := d.PingContext(ctx)
	if err != nil {
		h.Status = datasource.StatusDown

		return &h
	}

	h.Status = datasource.StatusUp

	dbStats := d.Stats()
	h.Details["stats"] = DBStats{
		MaxOpenConnections: dbStats.MaxOpenConnections,
		OpenConnections:    dbStats.OpenConnections,
		InUse:              dbStats.InUse,
		Idle:               dbStats.Idle,
		WaitCount:          dbStats.WaitCount,
		WaitDuration:       dbStats.WaitDuration,
		MaxIdleClosed:      dbStats.MaxIdleClosed,
		MaxIdleTimeClosed:  dbStats.MaxIdleTimeClosed,
		MaxLifetimeClosed:  dbStats.MaxLifetimeClosed,
	}

	return &h
}
