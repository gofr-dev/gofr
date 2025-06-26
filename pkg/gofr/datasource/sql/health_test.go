package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logging"
)

func TestHealth_HealthCheck(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	mock.ExpectPing()

	db.config = &DBConfig{
		HostName: "host",
		Port:     "3306",
		Database: "test",
	}

	expected := &datasource.Health{
		Status: "UP",
		Details: map[string]any{
			"host": "host:3306/test",
			"stats": DBStats{
				MaxOpenConnections: db.Stats().MaxOpenConnections,
				OpenConnections:    db.Stats().OpenConnections,
				InUse:              db.Stats().InUse,
				Idle:               db.Stats().Idle,
				WaitCount:          db.Stats().WaitCount,
				WaitDuration:       db.Stats().WaitDuration,
				MaxIdleClosed:      db.Stats().MaxIdleClosed,
				MaxIdleTimeClosed:  db.Stats().MaxIdleTimeClosed,
				MaxLifetimeClosed:  db.Stats().MaxLifetimeClosed,
			},
		},
	}

	out := db.HealthCheck()

	assert.Equal(t, expected, out)
}

func TestHealth_HealthCheckDBNotConnected(t *testing.T) {
	db := &DB{
		config: &DBConfig{
			HostName: "host",
			Port:     "3306",
			Database: "test",
		},
	}

	expected := &datasource.Health{
		Status: "DOWN",
		Details: map[string]any{
			"host": "host:3306/test",
		},
	}

	out := db.HealthCheck()

	assert.Equal(t, expected, out)
}

func TestHealth_HealthCheckDBPingFailed(t *testing.T) {
	db, mock := getDB(t, logging.INFO)
	defer db.DB.Close()

	mock.ExpectPing().WillReturnError(errDB)

	db.config = &DBConfig{
		HostName: "host",
		Port:     "3306",
		Database: "test",
	}

	expected := &datasource.Health{
		Status: "DOWN",
		Details: map[string]any{
			"host": "host:3306/test",
		},
	}

	out := db.HealthCheck()

	assert.Equal(t, expected, out)
}
