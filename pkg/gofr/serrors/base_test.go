package serrors

import (
	"testing"
)

func TestErrorGetters(t *testing.T) {
	err := &Error{
		statusCode:         "E100",
		subStatusCode:      "E101",
		level:              Level(WARNING),
		retryable:          true,
		externalStatusCode: 400,
		externalMessage:    "Bad Request",
	}

	if got := err.Code(); got != "E100" {
		t.Errorf("Code() = %s; want %s", got, "E100")
	}
	if got := err.SubCode(); got != "E101" {
		t.Errorf("SubCode() = %s; want %s", got, "E101")
	}
	if got := err.Level(); got != "WARNING" {
		t.Errorf("Level() = %s; want %s", got, "WARN")
	}
	if got := err.Retryable(); got != true {
		t.Errorf("Retryable() = %v; want %v", got, true)
	}
	if got := err.ExternalStatus(); got != 400 {
		t.Errorf("ExternalStatus() = %d; want %d", got, 400)
	}
	if got := err.ExternalMessage(); got != "Bad Request" {
		t.Errorf("ExternalMessage() = %s; want %s", got, "Bad Request")
	}
}

func TestErrorWithSetters(t *testing.T) {
	err := &Error{meta: make(map[string]any)}

	err.
		WithStatusCode("E200").
		WithSubCode("E201").
		WithLevel(Level(ERROR)).
		WithRetryable(false).
		WithMeta("requestID", "abc123").
		WithMetaMulti(map[string]any{"traceID": "xyz789", "count": 3}).
		WithExternalStatus(502).
		WithExternalMessage("Bad Gateway")

	if err.statusCode != "E200" {
		t.Errorf("WithStatusCode failed, got %s", err.statusCode)
	}
	if err.subStatusCode != "E201" {
		t.Errorf("WithSubCode failed, got %s", err.subStatusCode)
	}
	if err.level.GetErrorLevel() != "ERROR" {
		t.Errorf("WithLevel failed, got %s", err.level.GetErrorLevel())
	}
	if err.retryable != false {
		t.Errorf("WithRetryable failed, got %v", err.retryable)
	}
	if v := err.meta["requestID"]; v != "abc123" {
		t.Errorf("WithMeta failed, got %v", v)
	}
	if v := err.meta["traceID"]; v != "xyz789" {
		t.Errorf("WithMetaMulti failed for traceID, got %v", v)
	}
	if v := err.meta["count"]; v != 3 {
		t.Errorf("WithMetaMulti failed for count, got %v", v)
	}
	if err.externalStatusCode != 502 {
		t.Errorf("WithExternalStatus failed, got %d", err.externalStatusCode)
	}
	if err.externalMessage != "Bad Gateway" {
		t.Errorf("WithExternalMessage failed, got %s", err.externalMessage)
	}
}

func TestGetMetaString(t *testing.T) {
	meta := map[string]any{
		"user":    "alice",
		"attempt": 2,
	}
	got := getMetaString(meta)
	expected1 := `{"attempt":2,"user":"alice"}`
	expected2 := `{"user":"alice","attempt":2}` // JSON order can vary

	if got != expected1 && got != expected2 {
		t.Errorf("getMetaString() = %s; want %s or %s", got, expected1, expected2)
	}
}
