package pinecone

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryLogPrettyPrint_WithData(t *testing.T) {
	queryLog := QueryLog{
		Operation:   "query",
		Duration:    12345678, // 12.34ms
		Index:       "test-index",
		Namespace:   "test-namespace",
		VectorCount: 10,
		TopK:        5,
		Filter:      map[string]any{"category": "electronics", "price": map[string]any{"$lt": 100}},
		IDs:         []string{"id1", "id2", "id3", "id4", "id5", "id6", "id7"},
	}

	var buf bytes.Buffer
	queryLog.PrettyPrint(&buf)
	output := buf.String()

	// Verify expected information is in the output
	assert.Contains(t, output, "PINECONE")
	assert.Contains(t, output, "query")
	assert.Contains(t, output, "index:test-index")
	assert.Contains(t, output, "namespace:test-namespace")
	assert.Contains(t, output, "vectors:10")
	assert.Contains(t, output, "topK:5")
	assert.Contains(t, output, "ids:id1,id2,id3,id4,id5")
	assert.Contains(t, output, "filter:{\"category\":\"electronics\"")
	assert.Contains(t, output, "12.35s") // Rounded duration
}

func TestQueryLogPrettyPrint_Minimal(t *testing.T) {
	queryLog := QueryLog{
		Operation: "list_indexes",
		Duration:  500, // 500µs
	}

	var buf bytes.Buffer
	queryLog.PrettyPrint(&buf)
	output := buf.String()

	// Verify expected information is in the output
	assert.Contains(t, output, "PINECONE")
	assert.Contains(t, output, "list_indexes")
	assert.Contains(t, output, "500µs")

	// Verify fields that shouldn't be present
	assert.NotContains(t, output, "index:")
	assert.NotContains(t, output, "namespace:")
	assert.NotContains(t, output, "vectors:")
}

func TestQueryLogPrettyPrint_WithError(t *testing.T) {
	queryLog := QueryLog{
		Operation: "delete_index",
		Duration:  345678, // 345.68ms
		Index:     "nonexistent-index",
		Error:     "index not found",
	}

	var buf bytes.Buffer
	queryLog.PrettyPrint(&buf)
	output := buf.String()

	// Verify expected information is in the output
	assert.Contains(t, output, "PINECONE")
	assert.Contains(t, output, "delete_index")
	assert.Contains(t, output, "345.68ms")
	assert.Contains(t, output, "index:nonexistent-index")
	assert.Contains(t, output, "error:index not found")
}
