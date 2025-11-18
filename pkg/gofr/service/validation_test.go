package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationMode_String(t *testing.T) {
	assert.Equal(t, ValidationModeSoft, ValidationMode(0))
	assert.Equal(t, ValidationModeStrict, ValidationMode(1))
}

func TestValidateOptions_SoftMode(t *testing.T) {
	tests := []struct {
		name          string
		options       []Options
		expectWarning bool
		expectError   bool
	}{
		{
			name:          "valid options",
			options:       []Options{&APIKeyConfig{APIKey: "valid-key"}},
			expectWarning: false,
			expectError:   false,
		},
		{
			name:          "invalid option",
			options:       []Options{&APIKeyConfig{APIKey: ""}},
			expectWarning: true,
			expectError:   false,
		},
		{
			name: "multiple invalid options",
			options: []Options{
				&APIKeyConfig{APIKey: ""},
				&BasicAuthConfig{UserName: "", Password: ""},
			},
			expectWarning: true,
			expectError:   false,
		},
		{
			name: "mixed valid and invalid options",
			options: []Options{
				&APIKeyConfig{APIKey: "valid-key"},
				&APIKeyConfig{APIKey: ""},
			},
			expectWarning: true,
			expectError:   false,
		},
		{
			name:          "non-validator option",
			options:       []Options{&RetryConfig{MaxRetries: 3}},
			expectWarning: false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := logging.NewMockLogger(logging.DEBUG)
			logsBefore := len(mockLogger.Logs())

			config := ValidationConfig{
				Mode:   ValidationModeSoft,
				Logger: mockLogger,
			}

			err := validateOptions(config, tt.options)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectWarning {
				assert.Greater(t, len(mockLogger.Logs()), logsBefore, "Expected warning log")
			} else {
				assert.Equal(t, len(mockLogger.Logs()), logsBefore, "No warning log expected")
			}
		})
	}
}

func TestValidateOptions_StrictMode(t *testing.T) {
	tests := []struct {
		name        string
		options     []Options
		expectError bool
	}{
		{
			name:        "valid options",
			options:     []Options{&APIKeyConfig{APIKey: "valid-key"}},
			expectError: false,
		},
		{
			name:        "invalid option",
			options:     []Options{&APIKeyConfig{APIKey: ""}},
			expectError: true,
		},
		{
			name: "multiple invalid options",
			options: []Options{
				&APIKeyConfig{APIKey: ""},
				&BasicAuthConfig{UserName: "", Password: ""},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := logging.NewMockLogger(logging.DEBUG)
			config := ValidationConfig{
				Mode:   ValidationModeStrict,
				Logger: mockLogger,
			}

			err := validateOptions(config, tt.options)

			if tt.expectError {
				require.Error(t, err)
				validationErr, ok := err.(*ValidationErrors)
				assert.True(t, ok, "Error should be ValidationErrors")
				assert.Greater(t, len(validationErr.Errors), 0)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := ValidationError{
		Feature: "Test Feature",
		Err:     fmt.Errorf("test error"),
	}

	expected := "validation failed for Test Feature: test error"
	assert.Equal(t, expected, err.Error())
}

func TestValidationErrors_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   []ValidationError
		expected string
	}{
		{
			name:     "single error",
			errors:   []ValidationError{{Feature: "Feature1", Err: fmt.Errorf("error1")}},
			expected: "validation failed for Feature1: error1",
		},
		{
			name: "multiple errors",
			errors: []ValidationError{
				{Feature: "Feature1", Err: fmt.Errorf("error1")},
				{Feature: "Feature2", Err: fmt.Errorf("error2")},
			},
			expected: "2 validation errors: validation failed for Feature1: error1; validation failed for Feature2: error2",
		},
		{
			name:     "no errors",
			errors:   []ValidationError{},
			expected: "no validation errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := &ValidationErrors{Errors: tt.errors}
			assert.Equal(t, tt.expected, errs.Error())
		})
	}
}

func TestNewHTTPServiceWithValidation_SoftMode(t *testing.T) {
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Invalid option should log warning but service should still be created
	invalidConfig := &APIKeyConfig{APIKey: ""}
	svc := NewHTTPServiceWithValidation(
		server.URL,
		mockLogger,
		nil,
		ValidationConfig{Mode: ValidationModeSoft, Logger: mockLogger},
		invalidConfig,
	)

	// Service should be created despite invalid config
	assert.NotNil(t, svc)

	// Should have logged warning
	logs := mockLogger.Logs()
	assert.Greater(t, len(logs), 0, "Expected warning log")

	// Service should still function (though with invalid config)
	resp, err := svc.Get(context.Background(), "test", nil)
	// Service creation succeeded, but actual request behavior depends on implementation
	_ = resp
	_ = err
}

func TestNewHTTPServiceWithValidation_StrictMode(t *testing.T) {
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	tests := []struct {
		name        string
		options     []Options
		expectError bool
	}{
		{
			name:        "valid options",
			options:     []Options{&APIKeyConfig{APIKey: "valid-key"}},
			expectError: false,
		},
		{
			name:        "invalid option",
			options:     []Options{&APIKeyConfig{APIKey: ""}},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewHTTPServiceWithValidation(
				"http://example.com",
				mockLogger,
				nil,
				ValidationConfig{Mode: ValidationModeStrict, Logger: mockLogger},
				tt.options...,
			)

			if tt.expectError {
				// In strict mode with invalid options, service should fail on first request
				resp, err := svc.Get(context.Background(), "test", nil)
				assert.Error(t, err, "Expected error on first request")
				assert.Nil(t, resp)
				assert.Contains(t, err.Error(), "service creation failed")
			} else {
				// Valid service should be created
				assert.NotNil(t, svc)
			}
		})
	}
}

func TestValidationFailedService(t *testing.T) {
	err := fmt.Errorf("validation error")
	svc := &validationFailedService{err: err}

	tests := []struct {
		name        string
		method      func() error
		expectError bool
	}{
		{
			name: "Get",
			method: func() error {
				_, err := svc.Get(context.Background(), "test", nil)
				return err
			},
			expectError: true,
		},
		{
			name: "Post",
			method: func() error {
				_, err := svc.Post(context.Background(), "test", nil, nil)
				return err
			},
			expectError: true,
		},
		{
			name: "HealthCheck",
			method: func() error {
				health := svc.HealthCheck(context.Background())
				if health.Status == "DOWN" {
					return fmt.Errorf(health.Reason)
				}
				return nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.method()
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "validation error")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorImplementations(t *testing.T) {
	tests := []struct {
		name      string
		validator Validator
		expectErr bool
	}{
		{
			name:      "APIKeyConfig valid",
			validator: &APIKeyConfig{APIKey: "valid-key"},
			expectErr: false,
		},
		{
			name:      "APIKeyConfig invalid",
			validator: &APIKeyConfig{APIKey: ""},
			expectErr: true,
		},
		{
			name:      "BasicAuthConfig valid",
			validator: &BasicAuthConfig{UserName: "user", Password: "pass"},
			expectErr: false,
		},
		{
			name:      "BasicAuthConfig invalid username",
			validator: &BasicAuthConfig{UserName: "", Password: "pass"},
			expectErr: true,
		},
		{
			name:      "BasicAuthConfig invalid password",
			validator: &BasicAuthConfig{UserName: "user", Password: ""},
			expectErr: true,
		},
		{
			name: "OAuthConfig valid",
			validator: &OAuthConfig{
				ClientID:     "client-id",
				ClientSecret: "secret",
				TokenURL:     "https://example.com/token",
			},
			expectErr: false,
		},
		{
			name: "OAuthConfig invalid client ID",
			validator: &OAuthConfig{
				ClientID:     "",
				ClientSecret: "secret",
				TokenURL:     "https://example.com/token",
			},
			expectErr: true,
		},
		{
			name:      "DefaultHeaders valid",
			validator: &DefaultHeaders{Headers: map[string]string{"key": "value"}},
			expectErr: false,
		},
		{
			name:      "DefaultHeaders nil",
			validator: &DefaultHeaders{Headers: nil},
			expectErr: true,
		},
		{
			name:      "DefaultHeaders empty",
			validator: &DefaultHeaders{Headers: map[string]string{}},
			expectErr: true,
		},
		{
			name:      "CircuitBreakerConfig valid",
			validator: &CircuitBreakerConfig{Threshold: 5, Interval: time.Second},
			expectErr: false,
		},
		{
			name:      "CircuitBreakerConfig invalid threshold",
			validator: &CircuitBreakerConfig{Threshold: 0, Interval: time.Second},
			expectErr: true,
		},
		{
			name:      "CircuitBreakerConfig invalid interval",
			validator: &CircuitBreakerConfig{Threshold: 5, Interval: 0},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validator.Validate()
			if tt.expectErr {
				assert.Error(t, err, "Expected validation error")
			} else {
				assert.NoError(t, err, "Expected no validation error")
			}

			// Test FeatureName is not empty
			featureName := tt.validator.FeatureName()
			assert.NotEmpty(t, featureName, "FeatureName should not be empty")
		})
	}
}

func TestNewHTTPService_BackwardCompatibility(t *testing.T) {
	// Test that existing NewHTTPService still works (defaults to soft mode)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Should work with valid options
	validConfig, _ := NewAPIKeyConfig("valid-key")
	svc := NewHTTPService(server.URL, mockLogger, nil, validConfig)
	assert.NotNil(t, svc)

	// Should still work with invalid options (soft mode default)
	invalidConfig := &APIKeyConfig{APIKey: ""}
	svc2 := NewHTTPService(server.URL, mockLogger, nil, invalidConfig)
	assert.NotNil(t, svc2) // Service created despite invalid config (soft mode)
}

