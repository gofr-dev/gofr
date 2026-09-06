package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/datasource/file"
	gofrS3 "gofr.dev/pkg/gofr/datasource/file/s3"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// Defaults mirror examples/using-s3-filestore/configs/.env. S3_ENDPOINT can be
// overridden (e.g. to reach a MinIO container by service name) so the same test
// runs against localhost in CI and container-to-container locally.
const (
	testBucket    = "gofr-bucket"
	testAccessKey = "minioadmin"
	testSecretKey = "minioadmin"
	testRegion    = "us-east-1"
)

func testEndpoint() string {
	if e := os.Getenv("S3_ENDPOINT"); e != "" {
		return e
	}

	return "http://localhost:9000"
}

// TestS3FileStore_RoundTrip drives the example end to end against a real MinIO
// backend. It exists to catch two classes of bug that the s3 datasource's
// mock-based unit tests structurally cannot:
//
//   - a large object read back over a real HTTP body (Read must not truncate /
//     zero-fill — gofr-dev/gofr#3804 defect 1), and
//   - the first write under a brand-new prefix (Create must not require a
//     pre-existing parent — defect 2).
func TestS3FileStore_RoundTrip(t *testing.T) {
	skipIfMinIOUnreachable(t)

	sdk := newSDKClient(t)
	ensureBucket(t, sdk)

	configs := testutil.NewServerConfigs(t)
	t.Setenv("GOFR_TELEMETRY", "false")

	go main()
	waitForHTTPServer(t, configs.HTTPHost)

	// Defect 1: an object larger than one HTTP transport chunk must round-trip
	// byte-for-byte. The pre-fix Read filled only the first few KB of the buffer.
	large := strings.Repeat("gofr-s3-filestore-", 12000) // ~216 KiB
	upload(t, configs.HTTPHost, "big.txt", large)
	require.Equal(t, large, download(t, configs.HTTPHost, "big.txt"),
		"large object must round-trip without truncation or zero-fill")

	// A small object must also come back exactly, with no trailing zeros.
	upload(t, configs.HTTPHost, "small.txt", "hello gofr")
	require.Equal(t, "hello gofr", download(t, configs.HTTPHost, "small.txt"))

	// Defect 2: the first object under a never-before-used prefix must succeed.
	freshKey := fmt.Sprintf("uploads/%d/report.txt", time.Now().UnixNano())
	upload(t, configs.HTTPHost, freshKey, "under a fresh prefix")
	require.Equal(t, "under a fresh prefix", download(t, configs.HTTPHost, freshKey),
		"create under a fresh prefix must succeed and round-trip")
}

// TestS3FileStore_CopyAndReadAt exercises the read paths the HTTP round-trip
// (io.ReadAll) doesn't. io.Copy drives sequential Read calls, each issuing a
// byte-range request from a growing offset, and ReadAt fetches an exact window.
// Both run against real MinIO — the transport the mock unit tests can't provide —
// so a regression in the range logic (e.g. gofr-dev/gofr#3805 item 1, a backend
// answer that ignores Range) surfaces here rather than as silent corruption.
func TestS3FileStore_CopyAndReadAt(t *testing.T) {
	skipIfMinIOUnreachable(t)

	sdk := newSDKClient(t)
	ensureBucket(t, sdk)

	fs := newFileStore()

	// An object spanning many transport chunks so io.Copy makes several ranged
	// Read calls rather than a single one.
	content := strings.Repeat("gofr-range-", 20000) // ~220 KiB
	key := fmt.Sprintf("copy/%d/data.txt", time.Now().UnixNano())

	w, err := fs.Create(key)
	require.NoError(t, err)
	_, err = w.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	// io.Copy: sequential ranged reads must reassemble the object byte-for-byte
	// and terminate cleanly at EOF (never loop, never zero-fill).
	r, err := fs.Open(key)
	require.NoError(t, err)

	defer r.Close()

	var buf bytes.Buffer

	n, err := io.Copy(&buf, r)
	require.NoError(t, err, "io.Copy over a real HTTP body must not error or hang")
	require.Equal(t, int64(len(content)), n)
	require.Equal(t, content, buf.String(), "io.Copy must reassemble the object byte-for-byte")

	// ReadAt: an exact window in the middle of the object must return exactly
	// those bytes.
	ra, err := fs.Open(key)
	require.NoError(t, err)

	defer ra.Close()

	const off, size = 1000, 500

	window := make([]byte, size)

	got, err := ra.ReadAt(window, off)
	require.NoError(t, err)
	require.Equal(t, size, got)
	require.Equal(t, content[off:off+size], string(window), "ReadAt must return the requested window")
}

// newFileStore builds the gofr S3 file store the example uses, wired to the test
// MinIO backend, so a test can drive File methods (io.Copy, ReadAt) directly
// rather than only through the HTTP handlers.
func newFileStore() file.CloudFileSystem {
	endpoint := testEndpoint()

	fs := gofrS3.New(&gofrS3.Config{
		EndPoint:        endpoint,
		BucketName:      testBucket,
		Region:          testRegion,
		AccessKeyID:     testAccessKey,
		SecretAccessKey: testSecretKey,
		Flavor:          gofrS3.FlavorMinIO,
	})

	fs.UseLogger(logging.NewLogger(logging.ERROR))
	fs.Connect()

	return fs
}

// upload POSTs content to /files?name=<name> and asserts a 2xx response.
func upload(t *testing.T, host, name, content string) {
	t.Helper()

	body, err := json.Marshal(fileContent{Content: content})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		host+"/files?name="+url.QueryEscape(name), bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "upload request failed")

	defer resp.Body.Close()

	require.Less(t, resp.StatusCode, http.StatusBadRequest,
		"upload of %q should succeed, got status %d", name, resp.StatusCode)
}

// download GETs /files?name=<name> and returns the object's content.
func download(t *testing.T, host, name string) string {
	t.Helper()

	resp, err := http.Get(host + "/files?name=" + url.QueryEscape(name))
	require.NoError(t, err, "download request failed")

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "download of %q should return 200", name)

	var env struct {
		Data fileContent `json:"data"`
	}

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))

	return env.Data.Content
}

// waitForHTTPServer blocks until the server at host answers its liveness probe,
// and fails the test if it never does. It replaces a fixed sleep after
// `go main()`: a sleep long enough for a loaded CI runner is wasted on every
// other run, and one short enough to be cheap races the listener.
//
// pkg/gofr/testutil has the same helper, and this is a copy of it — deliberately.
// This directory is a separate module whose go.mod requires the released
// gofr.dev v1.57.0 with no `replace`, and that release's testutil has no
// WaitForHTTPServer. Calling the shared one would compile only inside the
// workspace and break `GOWORK=off go build ./...` here. Drop this in favor of
// testutil.WaitForHTTPServer once this module's gofr.dev requirement moves to a
// release that carries it.
func waitForHTTPServer(t *testing.T, host string) {
	t.Helper()

	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, host+"/.well-known/alive", http.NoBody)
		if err != nil {
			return false
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}

		defer resp.Body.Close()

		return resp.StatusCode == http.StatusOK
	}, 30*time.Second, 100*time.Millisecond, "HTTP server at %s did not start in time", host)
}

// newSDKClient builds a raw S3 client used only for test setup (bucket
// creation), independent of the code under test.
func newSDKClient(t *testing.T) *s3.Client {
	t.Helper()

	cfg, err := awsConfig.LoadDefaultConfig(context.Background(),
		awsConfig.WithRegion(testRegion),
		awsConfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(testAccessKey, testSecretKey, "")))
	require.NoError(t, err)

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

// ensureBucket creates the test bucket if it does not already exist.
func ensureBucket(t *testing.T, sdk *s3.Client) {
	t.Helper()

	_, err := sdk.CreateBucket(context.Background(),
		&s3.CreateBucketInput{Bucket: aws.String(testBucket)})
	if err != nil && !strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") &&
		!strings.Contains(err.Error(), "BucketAlreadyExists") {
		require.NoError(t, err, "failed to create test bucket")
	}
}

// skipIfMinIOUnreachable keeps local `go test` runs green when no MinIO is
// running. On developer machines the backend is optional and the test skips.
// CI sets S3_REQUIRE_MINIO so an unreachable backend fails the test instead of
// skipping — otherwise a broken MinIO would silently turn the #3804 integration
// guard into a no-op that still reports success.
func skipIfMinIOUnreachable(t *testing.T) {
	t.Helper()

	u, err := url.Parse(testEndpoint())
	require.NoError(t, err)

	host := u.Host
	if u.Port() == "" {
		host = net.JoinHostPort(u.Hostname(), "9000")
	}

	conn, err := net.DialTimeout("tcp", host, 500*time.Millisecond)
	if err != nil {
		if required, _ := strconv.ParseBool(os.Getenv("S3_REQUIRE_MINIO")); required {
			require.NoError(t, err, "S3_REQUIRE_MINIO is set but MinIO is unreachable at %s", host)
		}

		t.Skipf("MinIO not reachable at %s, skipping integration test: %v", host, err)
	}

	_ = conn.Close()
}
