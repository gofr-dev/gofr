package middleware

import (
	"context"
	"net/http"

	newrelic "github.com/newrelic/go-agent" //nolint:staticcheck // go-agent is deprecated
)

type newRelicTxnConst int

const newRelicTxnKey newRelicTxnConst = 0

// NewRelic is a middleware that registers all the endpoints and sends the data to newrelic server
func NewRelic(appname, license string) func(http.Handler) http.Handler {
	nrconfig := newrelic.NewConfig(appname, license)
	app, _ := newrelic.NewApplication(nrconfig)

	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			txn := app.StartTransaction(r.URL.Path, w, r)
			defer func() {
				_ = txn.End()
			}()
			ctx := context.WithValue(r.Context(), newRelicTxnKey, txn)
			*r = *r.Clone(ctx)
			inner.ServeHTTP(w, r)
		})
	}
}

// newRelicTxn creates a newrelic transaction
func newRelicTxn(ctx context.Context) (newrelic.Transaction, bool) {
	txn, ok := ctx.Value(newRelicTxnKey).(newrelic.Transaction)
	return txn, ok
}

// newRelicError reports the error to newRelic when an error happens
func newRelicError(ctx context.Context, e error) {
	txn, ok := newRelicTxn(ctx)
	if ok {
		_ = txn.NoticeError(e)
	}
}
