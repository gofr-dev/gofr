package pinecone

import (
	"context"
	"fmt"

	"github.com/pinecone-io/go-pinecone/v3/pinecone"
)

// indexManager handles index operations
type indexManager struct {
	client *Client
}

// newIndexManager creates a new index manager
func newIndexManager(client *Client) *indexManager {
	return &indexManager{client: client}
}

// listIndexes returns all available indexes in the Pinecone project
func (im *indexManager) listIndexes(ctx context.Context) ([]string, error) {
	opCtx := im.client.spanManager.setupOperation(ctx, "list_indexes")
	defer im.client.spanManager.cleanup(opCtx)

	if err := im.client.validateConnection(); err != nil {
		return nil, err
	}

	indexes, err := im.client.client.ListIndexes(opCtx.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}

	return im.extractIndexNames(indexes), nil
}

// describeIndex retrieves detailed information about a specific index
func (im *indexManager) describeIndex(ctx context.Context, indexName string) (map[string]any, error) {
	opCtx := im.client.spanManager.setupOperation(ctx, "describe_index")
	defer im.client.spanManager.cleanup(opCtx)

	im.client.spanManager.setSpanAttributes(opCtx.span, SpanAttributes{Index: indexName})

	if err := im.client.validateConnection(); err != nil {
		return nil, err
	}

	index, err := im.client.client.DescribeIndex(opCtx.ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to describe index %s: %w", indexName, err)
	}

	return im.buildIndexDescription(index), nil
}

// createIndex creates a new Pinecone index with the given parameters
func (im *indexManager) createIndex(ctx context.Context, indexName string, dimension int, metric string, options map[string]any) error {
	opCtx := im.client.spanManager.setupOperation(ctx, "create_index")
	defer im.client.spanManager.cleanup(opCtx)

	im.client.spanManager.setSpanAttributes(opCtx.span, SpanAttributes{
		Index:     indexName,
		Dimension: dimension,
		Metric:    metric,
	})

	if err := im.client.validateConnection(); err != nil {
		return err
	}

	req, err := im.buildCreateIndexRequest(indexName, dimension, metric, options)
	if err != nil {
		return err
	}

	_, err = im.client.client.CreateServerlessIndex(opCtx.ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create index %s: %w", indexName, err)
	}

	return nil
}

// deleteIndex deletes a Pinecone index
func (im *indexManager) deleteIndex(ctx context.Context, indexName string) error {
	opCtx := im.client.spanManager.setupOperation(ctx, "delete_index")
	defer im.client.spanManager.cleanup(opCtx)

	im.client.spanManager.setSpanAttributes(opCtx.span, SpanAttributes{Index: indexName})

	if err := im.client.validateConnection(); err != nil {
		return err
	}

	err := im.client.client.DeleteIndex(opCtx.ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to delete index %s: %w", indexName, err)
	}

	return nil
}

// Helper methods

// extractIndexNames extracts index names from the index objects
func (im *indexManager) extractIndexNames(indexes []*pinecone.Index) []string {
	indexNames := make([]string, 0, len(indexes))
	for _, index := range indexes {
		indexNames = append(indexNames, index.Name)
	}
	return indexNames
}

// buildIndexDescription builds a description map from the index object
func (im *indexManager) buildIndexDescription(index *pinecone.Index) map[string]any {
	return map[string]any{
		"name":      index.Name,
		"dimension": index.Dimension,
		"metric":    index.Metric,
		"host":      index.Host,
		"spec":      index.Spec,
		"status":    index.Status,
	}
}

// buildCreateIndexRequest creates a request for index creation
func (im *indexManager) buildCreateIndexRequest(indexName string, dimension int, metric string, options map[string]any) (*pinecone.CreateServerlessIndexRequest, error) {
	indexMetric, err := im.convertMetricType(metric)
	if err != nil {
		return nil, err
	}

	cloud, region := im.extractCloudOptions(options)
	dimension32 := int32(dimension)

	return &pinecone.CreateServerlessIndexRequest{
		Name:      indexName,
		Dimension: &dimension32,
		Metric:    &indexMetric,
		Cloud:     cloud,
		Region:    region,
	}, nil
}

// convertMetricType converts string metric to SDK metric type
func (im *indexManager) convertMetricType(metric string) (pinecone.IndexMetric, error) {
	switch metric {
	case metricCosine:
		return pinecone.Cosine, nil
	case metricEuclidean:
		return pinecone.Euclidean, nil
	case metricDotProduct:
		return pinecone.Dotproduct, nil
	default:
		return "", fmt.Errorf("unsupported metric: %s", metric)
	}
}

// extractCloudOptions extracts cloud and region from options
func (im *indexManager) extractCloudOptions(options map[string]any) (pinecone.Cloud, string) {
	cloud := pinecone.Aws
	region := defaultRegion

	if cloudStr, ok := options["cloud"].(string); ok {
		cloud = im.convertCloudType(cloudStr)
	}

	if regionStr, ok := options["region"].(string); ok {
		region = regionStr
	}

	return cloud, region
}

// convertCloudType converts string cloud to SDK cloud type
func (im *indexManager) convertCloudType(cloudStr string) pinecone.Cloud {
	switch cloudStr {
	case cloudGCP:
		return pinecone.Gcp
	case cloudAzure:
		return pinecone.Azure
	default:
		return pinecone.Aws
	}
}
