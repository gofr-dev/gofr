package couchbase

import (
	"encoding/json"
	"testing"

	"github.com/couchbase/gocb/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBucketWrapper_Handles(t *testing.T) {
	tests := []struct {
		name           string
		get            func(bw *bucketWrapper) collectionProvider
		wantScope      string
		wantCollection string
	}{
		{
			name:           "named collection in default scope",
			get:            func(bw *bucketWrapper) collectionProvider { return bw.Collection("users") },
			wantScope:      "_default",
			wantCollection: "users",
		},
		{
			name:           "default collection",
			get:            func(bw *bucketWrapper) collectionProvider { return bw.DefaultCollection() },
			wantScope:      "_default",
			wantCollection: "_default",
		},
		{
			name:           "collection from named scope",
			get:            func(bw *bucketWrapper) collectionProvider { return bw.Scope("inventory").Collection("items") },
			wantScope:      "inventory",
			wantCollection: "items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bw := &bucketWrapper{&gocb.Bucket{}}

			cw, ok := tt.get(bw).(*collectionWrapper)
			require.True(t, ok)

			assert.Equal(t, tt.wantScope, cw.ScopeName())
			assert.Equal(t, tt.wantCollection, cw.Name())
		})
	}
}

func TestResultWrappers_InvalidResult(t *testing.T) {
	tests := []struct {
		name   string
		result resultProvider
	}{
		{name: "query result without reader", result: &queryResultWrapper{&gocb.QueryResult{}}},
		{name: "analytics result without reader", result: &analyticsResultWrapper{&gocb.AnalyticsResult{}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var row json.RawMessage

			assert.False(t, tt.result.Next())
			require.EqualError(t, tt.result.Row(&row), "result object is no longer valid")
			require.EqualError(t, tt.result.Err(), "result object is no longer valid")
			require.EqualError(t, tt.result.Close(), "result object is no longer valid")
			assert.Nil(t, row)
		})
	}
}
