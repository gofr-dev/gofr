package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type NATSKVStore struct {
	js      nats.JetStreamContext
	kv      nats.KeyValue
	bucket  string
	tracer  trace.Tracer
	metrics Metrics
	logger  Logger
}

func NewNATSKVStore(nc *nats.Conn, bucketName string) (*NATSKVStore, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize JetStream: %w", err)
	}

	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: bucketName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create/access KV bucket: %w", err)
	}

	return &NATSKVStore{
		js:     js,
		kv:     kv,
		bucket: bucketName,
	}, nil
}

func (store *NATSKVStore) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		store.logger = l
	}
}

func (store *NATSKVStore) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		store.metrics = m
	}
}

func (store *NATSKVStore) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		store.tracer = t
	}
}

func (store *NATSKVStore) Get(ctx context.Context, key string) (string, error) {
	span := store.addTrace(ctx, "get", key)
	defer store.sendOperationStats(time.Now(), "GET", span, key)

	entry, err := store.kv.Get(key)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			return "", fmt.Errorf("key not found: %s", key)
		}
		return "", fmt.Errorf("failed to get key: %w", err)
	}

	return string(entry.Value()), nil
}

func (store *NATSKVStore) Set(ctx context.Context, key, value string) error {
	span := store.addTrace(ctx, "set", key)
	defer store.sendOperationStats(time.Now(), "SET", span, key, value)

	_, err := store.kv.Put(key, []byte(value))
	if err != nil {
		return fmt.Errorf("failed to set key-value pair: %w", err)
	}

	return nil
}

func (store *NATSKVStore) Delete(ctx context.Context, key string) error {
	span := store.addTrace(ctx, "delete", key)
	defer store.sendOperationStats(time.Now(), "DELETE", span, key)

	err := store.kv.Delete(key)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			return fmt.Errorf("key not found: %s", key)
		}
		return fmt.Errorf("failed to delete key: %w", err)
	}

	return nil
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (store *NATSKVStore) HealthCheck(context.Context) (any, error) {
	h := &Health{
		Details: make(map[string]any),
	}

	h.Details["bucket"] = store.bucket

	_, err := store.js.AccountInfo()
	if err != nil {
		h.Status = "DOWN"
		store.logger.Debugf("JetStream health check failed: %v", err)
		return h, fmt.Errorf("JetStream is not healthy: %w", err)
	}

	h.Status = "UP"
	return h, nil
}

func (store *NATSKVStore) sendOperationStats(start time.Time, methodType string, span trace.Span, kv ...string) {
	duration := time.Since(start).Microseconds()

	store.logger.Debug(&Log{
		Type:     methodType,
		Duration: duration,
		Key:      strings.Join(kv, " "),
	})

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("natskv.%v.duration(μs)", methodType), duration))
	}

	if store.metrics != nil {
		store.metrics.RecordHistogram(context.Background(), "app_natskv_stats", float64(duration), "method", methodType)
	}
}

func (store *NATSKVStore) addTrace(ctx context.Context, method, key string) trace.Span {
	if store.tracer != nil {
		_, span := store.tracer.Start(ctx, fmt.Sprintf("natskv-%v", method))
		span.SetAttributes(attribute.String("natskv.key", key))
		return span
	}
	return nil
}
