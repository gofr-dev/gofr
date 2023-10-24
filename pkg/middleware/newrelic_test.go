package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockHandlerForNewRelic struct{}

func (m *mockHandlerForNewRelic) ServeHTTP(http.ResponseWriter, *http.Request) {
}

func TestNewRelic(t *testing.T) {
	handler := NewRelic("gofr", "6378b0a5bf929e7eb36d480d4e3cd914b74eNRAL")(&mockHandlerForNewRelic{})
	req, _ := http.NewRequest("GET", "/hello", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	newRelicTxn := req.Context().Value(newRelicTxnKey)

	if newRelicTxn == nil {
		t.Error("NewRelicTxn not injected into the request")
	}
}

func TestNewRelicTxn(t *testing.T) {
	mockTxn := "MockValue"
	ctx := context.Background()
	ctx = context.WithValue(ctx, newRelicTxnKey, mockTxn)
	txn, ok := newRelicTxn(ctx)
	assert.Nilf(t, txn, "")
	assert.False(t, ok, "")
}

// TestNewRelicError tests the behavior when error occurs in newRelic
// Since this function does not return any value and does not have any formal parameter,
// we cannot achieve complete coverage for the newRelicError function.
func TestNewRelicError(*testing.T) {
	mockTxn := mockTransaction{}
	ctx := context.WithValue(context.Background(), newRelicTxnKey, mockTxn)
	err := errors.New("test error")

	newRelicError(ctx, err)
}

type mockTransaction struct {
}
