package surrealdb

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func Test_NewClient(t *testing.T) {
	config := &Config{
		Host:       "localhost",
		Port:       8000,
		Username:   "root",
		Password:   "root",
		Namespace:  "test_namespace",
		Database:   "test_database",
		TLSEnabled: false,
	}

	client := New(config)
	assert.NotNil(t, client)
	assert.Equal(t, config, client.config)
}

func Test_UseLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := New(&Config{})
	mockLogger := NewMockLogger(ctrl)

	t.Run("valid logger", func(t *testing.T) {
		client.UseLogger(mockLogger)
		assert.NotNil(t, client.logger)
	})
}

func Test_convertValue(t *testing.T) {
	client := &Client{}

	t.Run("float64 conversion", func(t *testing.T) {
		tests := []struct {
			name     string
			input    float64
			expected any
		}{
			{"valid float64", 42.0, 42},
			{"too large float64", math.MaxFloat64, nil},
			{"too small float64", -math.MaxFloat64, nil},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := client.convertValue(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("uint64 conversion", func(t *testing.T) {
		tests := []struct {
			name     string
			input    uint64
			expected any
		}{
			{"valid uint64", uint64(42), 42},
			{"too large uint64", uint64(math.MaxInt + 1), nil},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := client.convertValue(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("int64 conversion", func(t *testing.T) {
		tests := []struct {
			name     string
			input    int64
			expected any
		}{
			{"valid int64", int64(42), 42},
			{"at max boundary", int64(math.MaxInt), int(math.MaxInt)},
			{"at min boundary", int64(math.MinInt), int(math.MinInt)},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := client.convertValue(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("string conversion", func(t *testing.T) {
		input := "test string"
		result := client.convertValue(input)
		assert.Equal(t, input, result)
	})

	t.Run("default case", func(t *testing.T) {
		input := []int{1, 2, 3}
		result := client.convertValue(input)
		assert.Equal(t, input, result)
	})
}
