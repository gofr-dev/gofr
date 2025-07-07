package pinecone

import (
	"github.com/pinecone-io/go-pinecone/v3/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

// vectorConverter handles vector format conversions.
type vectorConverter struct{}

// newVectorConverter creates a new vector converter.
func newVectorConverter() *vectorConverter {
	return &vectorConverter{}
}

// convertVectorsToSDK converts interface vectors to SDK format.
func (vc *vectorConverter) convertVectorsToSDK(vectors []any) ([]*pinecone.Vector, error) {
	sdkVectors := make([]*pinecone.Vector, 0, len(vectors))

	for _, v := range vectors {
		sdkVector, err := vc.convertSingleVector(v)
		if err != nil {
			return nil, err
		}

		sdkVectors = append(sdkVectors, sdkVector)
	}

	return sdkVectors, nil
}

// convertSingleVector converts a single vector to SDK format.
func (*vectorConverter) convertSingleVector(v any) (*pinecone.Vector, error) {
	vec, ok := v.(Vector)
	if !ok {
		return nil, ErrInvalidVectorFormat
	}

	sdkVector := &pinecone.Vector{
		Id:     vec.ID,
		Values: &vec.Values,
	}

	if vec.Metadata != nil {
		if metadata, err := structpb.NewStruct(vec.Metadata); err == nil {
			sdkVector.Metadata = metadata
		}
	}

	return sdkVector, nil
}

// convertFetchResponse converts the SDK fetch response to the expected format.
func (*vectorConverter) convertFetchResponse(resp *pinecone.FetchVectorsResponse) map[string]any {
	results := make(map[string]any)

	for id, vector := range resp.Vectors {
		vec := Vector{ID: vector.Id}

		if vector.Values != nil {
			vec.Values = *vector.Values
		}

		if vector.Metadata != nil {
			vec.Metadata = vector.Metadata.AsMap()
		}

		results[id] = vec
	}

	return results
}

// processQueryResponse processes the query response and returns results.
func (vc *vectorConverter) processQueryResponse(resp *pinecone.QueryVectorsResponse) []any {
	results := make([]any, 0, len(resp.Matches))

	for _, match := range resp.Matches {
		result := vc.buildMatchResult(match)
		results = append(results, result)
	}

	return results
}

// buildMatchResult builds a result from a match.
func (*vectorConverter) buildMatchResult(match *pinecone.ScoredVector) map[string]any {
	result := map[string]any{
		"id":    match.Vector.Id,
		"score": match.Score,
	}

	if match.Vector.Values != nil {
		result["values"] = *match.Vector.Values
	}

	if match.Vector.Metadata != nil {
		result["metadata"] = match.Vector.Metadata.AsMap()
	}

	return result
}
