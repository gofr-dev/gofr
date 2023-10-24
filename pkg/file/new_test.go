package file

import (
	"fmt"
	"net"
	"net/textproto"
	"testing"

	"gofr.dev/pkg/errors"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
)

func TestNewFile(t *testing.T) {
	err := fmt.Errorf("google: could not find default credentials." +
		" See https://developers.google.com/accounts/docs/application-default-credentials for more information")

	testcases := []struct {
		store    string
		fileName string
		fileMode Mode
		expErr   error
	}{
		{Local, "test.txt", READ, nil},
		{Local, "test.txt", WRITE, nil},
		{Local, "test.txt", READWRITE, nil},
		{Local, "test.txt", APPEND, nil},
		{Azure, "test.txt", READ, nil},
		{Azure, "test.txt", WRITE, nil},
		{Azure, "test.txt", READWRITE, nil},
		{Azure, "test.txt", APPEND, nil},
		{AWS, "test.txt", READWRITE, nil},
		{GCP, "test.txt", WRITE, fmt.Errorf("dialing: %w", err)},
		{SFTP, "test.txt", READ, &net.OpError{}},
		{FTP, "test.txt", READ, &textproto.Error{}},
		{"invalid file storage", "test.txt", READ, errors.InvalidFileStorage},
	}
	for _, tc := range testcases {
		c := config.MockConfig{Data: map[string]string{
			"FILE_STORE": tc.store,
		}}
		_, err := NewWithConfig(&c, tc.fileName, tc.fileMode)

		assert.IsType(t, tc.expErr, err)
	}
}

// Test_setFTPConfig to test behavior of setFTPConfig
func Test_setFTPConfig(t *testing.T) {
	mockConfig := &config.MockConfig{Data: map[string]string{
		"FTP_HOST":           "localhost",
		"FTP_USER":           "user",
		"FTP_PASSWORD":       "pass",
		"FTP_PORT":           "20",
		"FTP_RETRY_DURATION": "5",
	}}
	mockConfigWithoutPort := &config.MockConfig{Data: map[string]string{
		"FTP_HOST":           "localhost",
		"FTP_USER":           "user",
		"FTP_PASSWORD":       "pass",
		"FTP_RETRY_DURATION": "5",
	}}
	mockConfigWithMissingConfigs := &config.MockConfig{Data: map[string]string{}}
	expConfig := FTPConfig{
		Host:          "localhost",
		User:          "user",
		Password:      "pass",
		Port:          20,
		RetryDuration: 5,
	}
	expConfigWithoutPort := FTPConfig{
		Host:          "localhost",
		User:          "user",
		Password:      "pass",
		Port:          21,
		RetryDuration: 5,
	}
	expConfigWithMissingConfgis := FTPConfig{
		Port:          21,
		RetryDuration: 5,
	}
	testCases := []struct {
		desc      string
		configs   *config.MockConfig
		expConfig FTPConfig
	}{
		{"Success case: all valid ftp configs provided", mockConfig, expConfig},
		{"Success case: FTP_PORT is missing, 21 will be used as default", mockConfigWithoutPort, expConfigWithoutPort},
		{"Success case: configs are missing", mockConfigWithMissingConfigs, expConfigWithMissingConfgis},
	}

	for i, tc := range testCases {
		configs := setFTPConfig(tc.configs)

		assert.Equalf(t, tc.expConfig, configs, "Test[%d] Failed: %v", i+1, tc.desc)
	}
}
