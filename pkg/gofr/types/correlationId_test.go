package types

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCorrelationIDFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), CorrelationIDKey, CorrelationID("b00ff8de800911ec8f6502bfe7568078"))

	correlationID := GetCorrelationIDFromContext(ctx)

	assert.Equalf(t, "b00ff8de800911ec8f6502bfe7568078", correlationID.String(),
		"TEST Failed. Expected : test-correlation-id Got : %v", correlationID)
}

func TestCorrelationID_Validate(t *testing.T) {
	var cID CorrelationID = "b00ff8de800911ec8f6502bfe7568078"

	err := cID.Validate()

	assert.Nil(t, err)
}

func TestCorrelationID_Validate_Failure(t *testing.T) {
	testCases := []struct {
		desc          string
		correlationID CorrelationID
	}{
		{"empty correlation id", ""},
		{"invalid correlation id", "shadtfe6373u1v"},
		{"nil correlation id", "00000000000000000000000000000000"},
	}

	for i, tc := range testCases {
		err := tc.correlationID.Validate()

		assert.NotNilf(t, err, "TEST[%d] Failed. Expected : not-nil Got : %v", i, err)
	}
}

func TestGetCorrelationIDFromContext_Success(t *testing.T) {
	correlationID := CorrelationID("b00ff8de800911ec8f6502bfe7568078")

	// Set the correlation ID in the context
	ctx := context.Background()
	newCtx, err := correlationID.SetInContext(ctx)

	// Retrieve the correlation ID from the new context
	retrievedID := GetCorrelationIDFromContext(newCtx).String()

	assert.Nil(t, err, "TEST Failed. Expected : %v Got : %v",
		nil, err)
	assert.Equal(t, retrievedID, correlationID.String(), "TEST[%d] Failed. Expected : %v Got : %v",
		retrievedID, correlationID.String())
}

func TestCorrelationID_SetInContext_Fail(t *testing.T) {
	correlationID := CorrelationID("shadtfe6373u1v")

	// Set the correlation ID in the context
	ctx := context.Background()
	newCtx, err := correlationID.SetInContext(ctx)

	// Retrieve the correlation ID from the new context
	retrievedID := GetCorrelationIDFromContext(newCtx).String()

	assert.Contains(t, err, "hex encoded trace-id must have length equals to 32", "TEST[%d] Failed")
	assert.Equalf(t, "<nil>", retrievedID, "TEST Failed")
}
func TestGenerateCorrelationID(t *testing.T) {
	// Create a context for testing
	ctx := context.Background()

	// Generate a correlation ID
	correlationID := GenerateCorrelationID(ctx)

	// Check if the generated correlation ID is valid
	err := correlationID.Validate()
	if err != nil {
		t.Errorf("Generated correlation ID is not valid: %v", err)
	}

	// Check if the generated correlation ID has the expected length
	expectedLength := correlationIDLength
	if len(correlationID) != expectedLength {
		t.Errorf("Expected correlation ID length: %d, got: %d", expectedLength, len(correlationID))
	}

	// Check if the generated correlation ID contains only hexadecimal characters
	validChars := "0123456789abcdef"
	for _, c := range correlationID {
		if !strings.ContainsRune(validChars, c) {
			t.Errorf("Correlation ID contains invalid character: %s", string(c))
			break
		}
	}
}
