package surrealdb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go"
)

func TestDBWrapper(t *testing.T) {
	tests := []struct {
		desc      string
		replies   map[string]any
		run       func(ctx context.Context, w *DBWrapper) (any, error)
		expResult any
		expErrMsg string
	}{
		{
			desc: "use sets namespace and database",
			run: func(ctx context.Context, w *DBWrapper) (any, error) {
				return nil, w.Use(ctx, "other_ns", "other_db")
			},
		},
		{
			desc:    "sign in returns token",
			replies: map[string]any{"signin": "jwt-token"},
			run: func(ctx context.Context, w *DBWrapper) (any, error) {
				return w.SignIn(ctx, &surrealdb.Auth{Username: "root", Password: "root"})
			},
			expResult: "jwt-token",
		},
		{
			desc:    "sign in failure",
			replies: map[string]any{"signin": rpcErrorReply{message: "invalid credentials"}},
			run: func(ctx context.Context, w *DBWrapper) (any, error) {
				return w.SignIn(ctx, &surrealdb.Auth{Username: "root", Password: "bad"})
			},
			expResult: "",
			expErrMsg: "invalid credentials",
		},
		{
			desc:    "info returns session information",
			replies: map[string]any{"info": map[string]any{"user": "root"}},
			run: func(ctx context.Context, w *DBWrapper) (any, error) {
				return w.Info(ctx)
			},
			expResult: map[string]any{"user": "root"},
		},
		{
			desc:    "info failure",
			replies: map[string]any{"info": rpcErrorReply{message: "not authenticated"}},
			run: func(ctx context.Context, w *DBWrapper) (any, error) {
				return w.Info(ctx)
			},
			expResult: map[string]any(nil),
			expErrMsg: "not authenticated",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			w := newTestDBWrapper(t, tc.replies)

			result, err := tc.run(t.Context(), w)

			assert.Equal(t, tc.expResult, result)
			assert.Equal(t, tc.expErrMsg, errMsg(err))
			assert.NotNil(t, w.GetDB())
		})
	}
}
