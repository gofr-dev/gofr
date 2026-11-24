package azure

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr/datasource/file"
)

// statusRecorder collects the status label of every recorded file-store operation.
type statusRecorder struct {
	mu       sync.Mutex
	statuses []string
}

func (*statusRecorder) NewHistogram(string, string, ...float64) {}

func (r *statusRecorder) RecordHistogram(_ context.Context, _ string, _ float64, labels ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i+1 < len(labels); i += 2 {
		if labels[i] == "status" {
			r.statuses = append(r.statuses, labels[i+1])
		}
	}
}

func (r *statusRecorder) got() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.statuses...)
}

// TestCommonFileSystem_Stat_MissingNameStatus checks that Stat of a name the share does not
// hold is recorded NOT_FOUND like Open, while a permission failure stays ERROR.
func TestCommonFileSystem_Stat_MissingNameStatus(t *testing.T) {
	tests := []struct {
		name         string
		httpStatus   int
		errorCode    string
		wantNotExist bool
		wantStatus   string
	}{
		{name: "resource not found", httpStatus: http.StatusNotFound, errorCode: "ResourceNotFound", wantNotExist: true, wantStatus: "NOT_FOUND"},
		{name: "parent not found", httpStatus: http.StatusNotFound, errorCode: "ParentNotFound", wantNotExist: true, wantStatus: "NOT_FOUND"},
		{name: "share not found", httpStatus: http.StatusNotFound, errorCode: "ShareNotFound", wantNotExist: true, wantStatus: "NOT_FOUND"},
		{name: "forbidden stays an error", httpStatus: http.StatusForbidden, errorCode: "AuthorizationFailure", wantStatus: file.StatusError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := setupAzureTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("x-ms-error-code", tt.errorCode)
				w.WriteHeader(tt.httpStatus)
			})
			defer srv.Close()

			shareClient, err := createTestShareClient(t, srv.URL)
			require.NoError(t, err)

			metrics := &statusRecorder{}
			fs := &azureFileSystem{
				CommonFileSystem: &file.CommonFileSystem{
					Provider:     &storageAdapter{cfg: &Config{ShareName: "testshare"}, shareClient: shareClient},
					Location:     "testshare",
					Metrics:      metrics,
					ProviderName: "Azure",
				},
			}
			fs.CommonFileSystem.SetDisableRetry(true)

			_, err = fs.CommonFileSystem.Stat("nope")

			require.Error(t, err)
			assert.Equal(t, tt.wantNotExist, errors.Is(err, os.ErrNotExist))
			assert.Equal(t, []string{tt.wantStatus}, metrics.got())
		})
	}
}
