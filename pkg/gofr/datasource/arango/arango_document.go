package arango

import (
	"context"
	"time"

	"github.com/arangodb/go-driver/v2/arangodb"
)

type Document struct {
	client *Client
}

// CreateDocument creates a new document in the specified collection.
func (d *Document) CreateDocument(ctx context.Context, dbName, collectionName string, document any) (string, error) {
	collection, tracerCtx, err := executeCollectionOperation(ctx, *d, dbName, collectionName,
		"createDocument", "")
	if err != nil {
		return "", err
	}

	meta, err := collection.CreateDocument(tracerCtx, document)
	if err != nil {
		return "", err
	}

	return meta.Key, nil
}

// GetDocument retrieves a document by its ID from the specified collection.
func (d *Document) GetDocument(ctx context.Context, dbName, collectionName, documentID string, result any) error {
	collection, tracerCtx, err := executeCollectionOperation(ctx, *d, dbName, collectionName,
		"getDocument", documentID)
	if err != nil {
		return err
	}

	_, err = collection.ReadDocument(tracerCtx, documentID, result)

	return err
}

// UpdateDocument updates an existing document in the specified collection.
func (d *Document) UpdateDocument(ctx context.Context, dbName, collectionName, documentID string, document any) error {
	collection, tracerCtx, err := executeCollectionOperation(ctx, *d, dbName, collectionName,
		"updateDocument", documentID)
	if err != nil {
		return err
	}

	_, err = collection.UpdateDocument(tracerCtx, documentID, document)

	return err
}

// DeleteDocument deletes a document by its ID from the specified collection.
func (d *Document) DeleteDocument(ctx context.Context, dbName, collectionName, documentID string) error {
	collection, tracerCtx, err := executeCollectionOperation(ctx, *d, dbName, collectionName,
		"deleteDocument", documentID)
	if err != nil {
		return err
	}

	_, err = collection.DeleteDocument(tracerCtx, documentID)

	return err
}

// CreateEdgeDocument creates a new edge document between two vertices.
func (d *Document) CreateEdgeDocument(ctx context.Context, dbName, collectionName, from, to string, document any) (string, error) {
	collection, tracerCtx, err := executeCollectionOperation(ctx, *d, dbName, collectionName,
		"createEdgeDocument", "")
	if err != nil {
		return "", err
	}

	meta, err := collection.CreateDocument(tracerCtx, map[string]any{
		"_from": from,
		"_to":   to,
		"data":  document,
	})
	if err != nil {
		return "", err
	}

	return meta.Key, nil
}

func executeCollectionOperation(ctx context.Context, d Document, dbName, collectionName,
	operation string, documentID string) (arangodb.Collection, context.Context, error) {
	tracerCtx, span := d.client.addTrace(ctx, operation, map[string]string{"collection": collectionName})
	startTime := time.Now()

	ql := &QueryLog{Operation: operation,
		Database:   dbName,
		Collection: collectionName}

	if documentID != "" {
		ql.ID = documentID
	}

	defer d.client.sendOperationStats(ql, startTime, operation, span)

	collection, err := d.client.DB.getCollection(tracerCtx, dbName, collectionName)
	if err != nil {
		return nil, nil, err
	}

	return collection, tracerCtx, nil
}
