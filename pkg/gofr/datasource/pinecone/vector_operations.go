package pinecone

import (
	"context"
	"fmt"

	"github.com/pinecone-io/go-pinecone/v3/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

// vectorOperations handles the actual vector operations implementation.
type vectorOperations struct {
	client    *Client
	converter *vectorConverter
}

// newVectorOperations creates a new vector operations handler.
func newVectorOperations(client *Client) *vectorOperations {
	return &vectorOperations{
		client:    client,
		converter: newVectorConverter(),
	}
}

// performUpsert handles the actual upsert operation.
func (vo *vectorOperations) performUpsert(ctx context.Context, indexName, namespace string, vectors []any) (int, error) {
	indexConn, err := vo.getIndexConnection(ctx, indexName, namespace)
	if err != nil {
		return 0, err
	}
	defer indexConn.Close()

	sdkVectors, err := vo.converter.convertVectorsToSDK(vectors)
	if err != nil {
		return 0, err
	}

	count, err := indexConn.UpsertVectors(ctx, sdkVectors)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert vectors to index %s: %w", indexName, err)
	}

	return int(count), nil
}

// performQuery handles the actual query operation.
func (vo *vectorOperations) performQuery(ctx context.Context, params *QueryParams) ([]any, error) {
	indexConn, err := vo.getIndexConnection(ctx, params.IndexName, params.Namespace)
	if err != nil {
		return nil, err
	}

	defer indexConn.Close()

	req := vo.buildQueryRequest(params)
	resp, err := indexConn.QueryByVectorValues(ctx, req)

	if err != nil {
		return nil, fmt.Errorf("failed to query index %s: %w", params.IndexName, err)
	}

	return vo.converter.processQueryResponse(resp), nil
}

// performFetch handles the actual fetch operation.
func (vo *vectorOperations) performFetch(ctx context.Context, indexName, namespace string, ids []string) (map[string]any, error) {
	indexConn, err := vo.getIndexConnection(ctx, indexName, namespace)
	if err != nil {
		return nil, err
	}
	defer indexConn.Close()

	resp, err := indexConn.FetchVectors(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vectors from index %s: %w", indexName, err)
	}

	return vo.converter.convertFetchResponse(resp), nil
}

// performDelete handles the actual delete operation.
func (vo *vectorOperations) performDelete(ctx context.Context, indexName, namespace string, ids []string) error {
	indexConn, err := vo.getIndexConnection(ctx, indexName, namespace)
	if err != nil {
		return err
	}
	defer indexConn.Close()

	err = indexConn.DeleteVectorsById(ctx, ids)
	if err != nil {
		return fmt.Errorf("failed to delete vectors from index %s: %w", indexName, err)
	}

	return nil
}

// performDeleteAll handles the actual delete all operation.
func (vo *vectorOperations) performDeleteAll(ctx context.Context, indexName, namespace string) error {
	indexConn, err := vo.getIndexConnection(ctx, indexName, namespace)
	if err != nil {
		return err
	}
	defer indexConn.Close()

	err = indexConn.DeleteAllVectorsInNamespace(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete all vectors from index %s namespace %s: %w", indexName, namespace, err)
	}

	return nil
}

// getIndexConnection creates and returns an index connection.
func (vo *vectorOperations) getIndexConnection(ctx context.Context, indexName, namespace string) (*pinecone.IndexConnection, error) {
	indexDesc, err := vo.client.client.DescribeIndex(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to describe index %s: %w", indexName, err)
	}

	indexConn, err := vo.client.client.Index(pinecone.NewIndexConnParams{
		Host:      indexDesc.Host,
		Namespace: namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create index connection: %w", err)
	}

	return indexConn, nil
}

// buildQueryRequest creates a query request from parameters.
func (*vectorOperations) buildQueryRequest(params *QueryParams) *pinecone.QueryByVectorValuesRequest {
	topK := uint32(params.TopK) // #nosec G115 -- TopK is validated by caller
	if params.TopK > int(^uint32(0)>>1) {
		topK = ^uint32(0) >> 1 // Use max safe value
	}

	req := &pinecone.QueryByVectorValuesRequest{
		Vector: params.Vector,
		TopK:   topK,
	}

	if params.Filter != nil {
		if filterStruct, err := structpb.NewStruct(params.Filter); err == nil {
			req.MetadataFilter = filterStruct
		}
	}

	req.IncludeValues = params.IncludeValues
	req.IncludeMetadata = params.IncludeMetadata

	return req
}
