package file

import (
	"io"
	"testing"
	"time"

	pkgFtp "github.com/jlaffaye/ftp"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

// Test_retryFTP to test the behavior of retryFTP
func Test_retryFTP_connNil(t *testing.T) {
	ftpConfig := FTPConfig{
		Host:          "localhost",
		User:          "myuser",
		Password:      "mypass",
		Port:          21,
		RetryDuration: 1,
	}

	ftpInstance := &ftp{
		fileName: "test.txt",
		fileMode: "rw",
		conn:     nil, // Initialize conn to nil
	}

	// Start the retryFTP goroutine
	go retryFTP(&ftpConfig, ftpInstance)

	// Wait for the retryFTP goroutine to complete
	time.Sleep(2 * time.Second)

	_, err := ftpInstance.conn.List(".")

	assert.NoError(t, err, "Test Failed: Expected successful connection")
}

func Test_retryFTP_connNotNil(t *testing.T) {
	ftpConfig := FTPConfig{
		Host:          "localhost",
		User:          "myuser",
		Password:      "",
		Port:          21,
		RetryDuration: 5,
	}

	ftpInstance := ftp{fileName: "test.txt",
		fileMode: "rw",
	}

	_ = connectFTP(&ftpConfig, &ftpInstance)

	ftpConfig = FTPConfig{
		Host:          "localhost",
		User:          "myuser",
		Password:      "mypass",
		Port:          21,
		RetryDuration: 5,
	}

	go retryFTP(&ftpConfig, &ftpInstance)
	time.Sleep(1 * time.Minute)

	_, err := ftpInstance.conn.List(".")

	assert.NoError(t, err, "Test Failed: Expected successful connection")
}

func Test_retryFTP_TypeAssertionFail(t *testing.T) {
	var mockOp mockFtpOp

	ftpInstance := &ftp{fileName: "test.txt",
		fileMode: "rw",
		conn:     mockOp,
	}

	go retryFTP(&FTPConfig{RetryDuration: 2}, ftpInstance)
	time.Sleep(5 * time.Second)

	_, err := ftpInstance.conn.List(".")

	assert.Error(t, errors.Error("Test Error"), err, "Test Failed: Expected successful connection")
}

// Test_getRetryDuration to test the behavior of getRetryDuration
func Test_getRetryDuration(t *testing.T) {
	testCases := []struct {
		desc             string
		duration         string
		expRetryDuration time.Duration
	}{
		{"Case: valid duration passed", "10", time.Duration(10)},
		{"Case: invalid duration passed, default duration will be returned", "10.5", time.Duration(5)},
		{"Case: no duration passed, default duration will be returned", "", time.Duration(5)},
	}

	for i, tc := range testCases {
		retryDuration := getRetryDuration(tc.duration)

		assert.Equalf(t, tc.expRetryDuration, retryDuration, "Test [%d] Failed: %v", i+1, tc.desc)
	}
}

type mockFtpOp struct{}

func (m mockFtpOp) Read(string) (io.ReadCloser, error) {
	return nil, nil
}

func (m mockFtpOp) Write(string, io.Reader) error {
	return nil
}

func (m mockFtpOp) List(string) (entries []*pkgFtp.Entry, err error) {
	return nil, errors.Error("Test Error")
}

func (m mockFtpOp) Move(string, string) error {
	return nil
}

func (m mockFtpOp) Mkdir(string) error {
	return nil
}

func (m mockFtpOp) Close() error {
	return nil
}
