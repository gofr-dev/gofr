package pinecone

import (
	"context"
)

// vectorManager handles vector operations
type vectorManager struct {
	client     *Client
	operations *vectorOperations
}

// newVectorManager creates a new vector manager
func newVectorManager(client *Client) *vectorManager {
	return &vectorManager{
		client:     client,
		operations: newVectorOperations(client),
	}
}

// upsert adds or updates vectors in a specific namespace of an index
func (vm *vectorManager) upsert(ctx context.Context, indexName, namespace string, vectors []any) (int, error) {
	opCtx := vm.client.spanManager.setupOperation(ctx, "upsert")
	defer vm.client.spanManager.cleanup(opCtx)

	vm.client.spanManager.setSpanAttributes(opCtx.span, SpanAttributes{
		Index:     indexName,
		Namespace: namespace,
		Count:     len(vectors),
	})

	if err := vm.client.validateConnection(); err != nil {
		return 0, err
	}

	return vm.operations.performUpsert(opCtx.ctx, indexName, namespace, vectors)
}

// query searches for similar vectors in the index
func (vm *vectorManager) query(ctx context.Context, params QueryParams) ([]any, error) {
	opCtx := vm.client.spanManager.setupOperation(ctx, "query")
	defer vm.client.spanManager.cleanup(opCtx)

	vm.client.spanManager.setSpanAttributes(opCtx.span, SpanAttributes{
		Index:     params.IndexName,
		Namespace: params.Namespace,
		Count:     params.TopK,
	})

	if err := vm.client.validateConnection(); err != nil {
		return nil, err
	}

	return vm.operations.performQuery(opCtx.ctx, params)
}

// fetch retrieves vectors by their IDs
func (vm *vectorManager) fetch(ctx context.Context, indexName, namespace string, ids []string) (map[string]any, error) {
	opCtx := vm.client.spanManager.setupOperation(ctx, "fetch")
	defer vm.client.spanManager.cleanup(opCtx)

	vm.client.spanManager.setSpanAttributes(opCtx.span, SpanAttributes{
		Index:     indexName,
		Namespace: namespace,
		Count:     len(ids),
	})

	if err := vm.client.validateConnection(); err != nil {
		return nil, err
	}

	return vm.operations.performFetch(opCtx.ctx, indexName, namespace, ids)
}

// delete removes vectors from the index
func (vm *vectorManager) delete(ctx context.Context, indexName, namespace string, ids []string) error {
	opCtx := vm.client.spanManager.setupOperation(ctx, "delete")
	defer vm.client.spanManager.cleanup(opCtx)

	vm.client.spanManager.setSpanAttributes(opCtx.span, SpanAttributes{
		Index:     indexName,
		Namespace: namespace,
		Count:     len(ids),
	})

	if err := vm.client.validateConnection(); err != nil {
		return err
	}

	return vm.operations.performDelete(opCtx.ctx, indexName, namespace, ids)
}

// deleteAll removes all vectors from a namespace
func (vm *vectorManager) deleteAll(ctx context.Context, indexName, namespace string) error {
	opCtx := vm.client.spanManager.setupOperation(ctx, "delete_all")
	defer vm.client.spanManager.cleanup(opCtx)

	vm.client.spanManager.setSpanAttributes(opCtx.span, SpanAttributes{
		Index:     indexName,
		Namespace: namespace,
	})

	if err := vm.client.validateConnection(); err != nil {
		return err
	}

	return vm.operations.performDeleteAll(opCtx.ctx, indexName, namespace)
}
