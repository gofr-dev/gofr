package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestGetConfigs(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		keyAccessControlAllowOrigin:         "*",
		"ACCESS_CONTROL_ALLOW_HEADERS":      "Authorization, Content-Type",
		"ACCESS_CONTROL_ALLOW_CREDENTIALS":  "true",
		"ACCESS_CONTROL_ALLOW_CUSTOMHEADER": "abc",
	})

	middlewareConfigs := GetConfigs(mockConfig, logging.NewMockLogger(logging.WARN))

	expectedConfigs := map[string]string{
		"Access-Control-Allow-Origin":      "*",
		"Access-Control-Allow-Headers":     "Authorization, Content-Type",
		"Access-Control-Allow-Credentials": "true",
	}

	assert.Equal(t, expectedConfigs, middlewareConfigs.CorsHeaders, "TestGetConfigs Failed!")
	assert.NotContains(t, middlewareConfigs.CorsHeaders, "Access-Control-Allow-CustomHeader", "TestGetConfigs Failed!")
}

func TestLogDisableProbesConfig(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"LOG_DISABLE_PROBES": "true",
	})

	middlewareConfigs := GetConfigs(mockConfig, logging.NewMockLogger(logging.WARN))

	assert.True(t, middlewareConfigs.LogProbes.Disabled, "TestLogDisableProbesConfig Failed!")
}

func TestGetConfigs_ValidCORSValueIsKept(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  string
		header string
	}{
		{"zero max age", keyAccessControlMaxAge, "0", "Access-Control-Max-Age"},
		{"positive max age", keyAccessControlMaxAge, "600", "Access-Control-Max-Age"},
		{"credentials true", keyAccessControlAllowCredentials, "true", "Access-Control-Allow-Credentials"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockConfig := config.NewMockConfig(map[string]string{tc.key: tc.value})

			logs := testutil.StdoutOutputForFunc(func() {
				middlewareConfigs := GetConfigs(mockConfig, logging.NewMockLogger(logging.WARN))

				assert.Equal(t, tc.value, middlewareConfigs.CorsHeaders[tc.header])
			})

			assert.Empty(t, logs, "a valid value must not be reported as a misconfiguration")
		})
	}
}

// Browsers read these max-age spellings as a definite duration, so dropping them would
// replace the configured behavior with the 5 second default. They are rewritten to the
// canonical form, which every browser reads the same way, and the rewrite is reported.
func TestGetConfigs_NonCanonicalMaxAgeIsNormalized(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{"negative disables caching", "-1", "0"},
		{"large negative disables caching", "-3600", "0"},
		{"negative zero", "-0", "0"},
		{"explicit plus sign", "+600", "600"},
		{"zero padded", "0600", "600"},
		{"zero padded zero", "00", "0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockConfig := config.NewMockConfig(map[string]string{keyAccessControlMaxAge: tc.value})

			logs := testutil.StdoutOutputForFunc(func() {
				middlewareConfigs := GetConfigs(mockConfig, logging.NewMockLogger(logging.WARN))

				assert.Equal(t, tc.expected, middlewareConfigs.CorsHeaders["Access-Control-Max-Age"])
			})

			assert.Contains(t, logs, keyAccessControlMaxAge, "the warning must name the config key")
			assert.Contains(t, logs, tc.value, "the warning must name the configured value")
			assert.Contains(t, logs, "not in canonical form", "a rewrite must not be reported as a dropped header")
		})
	}
}

func TestGetConfigs_NonCanonicalMaxAgeWithoutLogger(t *testing.T) {
	middlewareConfigs := GetConfigs(config.NewMockConfig(map[string]string{keyAccessControlMaxAge: "-1"}))

	assert.Equal(t, "0", middlewareConfigs.CorsHeaders["Access-Control-Max-Age"])
}

func TestGetConfigs_InvalidCORSValueIsDroppedAndLogged(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  string
		header string
	}{
		{"max age with a duration unit", keyAccessControlMaxAge, "10m", "Access-Control-Max-Age"},
		{"max age with a seconds suffix", keyAccessControlMaxAge, "600s", "Access-Control-Max-Age"},
		{"non numeric max age", keyAccessControlMaxAge, "abc", "Access-Control-Max-Age"},
		{"max age overflowing int64", keyAccessControlMaxAge, "99999999999999999999", "Access-Control-Max-Age"},
		{"max age with surrounding space", keyAccessControlMaxAge, " 600", "Access-Control-Max-Age"},
		{"non boolean credentials", keyAccessControlAllowCredentials, "yes", "Access-Control-Allow-Credentials"},
		{"numeric credentials", keyAccessControlAllowCredentials, "1", "Access-Control-Allow-Credentials"},
		{"upper case credentials", keyAccessControlAllowCredentials, "TRUE", "Access-Control-Allow-Credentials"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockConfig := config.NewMockConfig(map[string]string{tc.key: tc.value})

			logs := testutil.StdoutOutputForFunc(func() {
				middlewareConfigs := GetConfigs(mockConfig, logging.NewMockLogger(logging.WARN))

				assert.NotContains(t, middlewareConfigs.CorsHeaders, tc.header,
					"an invalid value must not reach the response as a malformed header")
			})

			assert.Contains(t, logs, tc.key, "the warning must name the config key")
			assert.Contains(t, logs, tc.value, "the warning must name the offending value")
		})
	}
}

func TestGetConfigs_InvalidCORSValueWithoutLogger(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{keyAccessControlMaxAge: "10m"})

	middlewareConfigs := GetConfigs(mockConfig, nil)

	assert.NotContains(t, middlewareConfigs.CorsHeaders, "Access-Control-Max-Age")
}

// A browser honors Access-Control-Allow-Credentials only when it is exactly "true",
// so "false" is satisfied by omitting the header. That is an explicit opt-out rather
// than a misconfiguration, and must not be reported as one.
func TestGetConfigs_CredentialsFalseIsOmittedWithoutWarning(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{keyAccessControlAllowCredentials: "false"})

	logs := testutil.StdoutOutputForFunc(func() {
		middlewareConfigs := GetConfigs(mockConfig, logging.NewMockLogger(logging.WARN))

		assert.NotContains(t, middlewareConfigs.CorsHeaders, "Access-Control-Allow-Credentials")
	})

	assert.Empty(t, logs, "an explicit opt-out must not be reported as a misconfiguration")
}

// The logger is variadic so that existing callers of the exported GetConfigs keep
// compiling; an invalid value is still dropped when no logger is supplied.
func TestGetConfigs_WithoutLoggerArgument(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		keyAccessControlMaxAge:      "10m",
		keyAccessControlAllowOrigin: "*",
	})

	middlewareConfigs := GetConfigs(mockConfig)

	assert.NotContains(t, middlewareConfigs.CorsHeaders, "Access-Control-Max-Age")
	assert.Equal(t, "*", middlewareConfigs.CorsHeaders["Access-Control-Allow-Origin"])
}

func TestGetConfigs_MetricsCardinalityLimit(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected int
	}{
		{name: "neither set uses the SDK default", env: map[string]string{}, expected: 2000},
		{name: "METRICS_CARDINALITY_LIMIT", env: map[string]string{"METRICS_CARDINALITY_LIMIT": "100"}, expected: 100},
		{name: "OTEL_GO_X_CARDINALITY_LIMIT", env: map[string]string{"OTEL_GO_X_CARDINALITY_LIMIT": "300"}, expected: 300},
		{name: "GoFr key wins over the OTel key",
			env: map[string]string{"METRICS_CARDINALITY_LIMIT": "100", "OTEL_GO_X_CARDINALITY_LIMIT": "300"}, expected: 100},
		{name: "invalid GoFr key falls through to the OTel key",
			env: map[string]string{"METRICS_CARDINALITY_LIMIT": "lots", "OTEL_GO_X_CARDINALITY_LIMIT": "300"}, expected: 300},
		{name: "zero means unlimited and is kept", env: map[string]string{"METRICS_CARDINALITY_LIMIT": "0"}, expected: 0},
		{name: "whitespace is trimmed", env: map[string]string{"METRICS_CARDINALITY_LIMIT": " 64 "}, expected: 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetConfigs(config.NewMockConfig(tt.env)).MetricsCardinalityLimit)
		})
	}
}
