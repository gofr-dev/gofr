package pinecone

import "errors"

var (
	// ErrClientNotConnected indicates that the pinecone client is not connected.
	ErrClientNotConnected = errors.New("pinecone client not connected")

	// ErrUnsupportedMetric indicates that the provided metric is not supported.
	ErrUnsupportedMetric = errors.New("unsupported metric")

	// ErrInvalidVectorFormat indicates that the vector format is invalid.
	ErrInvalidVectorFormat = errors.New("invalid vector format")

	// ErrQuery indicates that a query operation failed.
	ErrQuery = errors.New("query error")

	// ErrDimensionOverflow indicates that the dimension overflows int32.
	ErrDimensionOverflow = errors.New("dimension overflows int32")

	// ErrDeleteVectorsFailed indicates that deleting vectors failed.
	ErrDeleteVectorsFailed = errors.New("failed to delete vectors")

	// ErrDeleteAllVectorsFailed indicates that deleting all vectors failed.
	ErrDeleteAllVectorsFailed = errors.New("failed to delete all vectors")

	// ErrInvalidAPIKey indicates that the API key is invalid.
	ErrInvalidAPIKey = errors.New("invalid API key")
)
