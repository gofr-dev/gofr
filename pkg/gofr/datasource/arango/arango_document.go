package arango

import (
	"context"
	"time"
)

type Document struct {
	client *Client
}

// CreateDocument creates a new document in the specified collection.
func (d *Document) CreateDocument(ctx context.Context, dbName, collectionName string, document any) (string, error) {
	tracerCtx, span := d.client.addTrace(ctx, "createDocument", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer d.client.sendOperationStats(&QueryLog{Query: "createDocument", Collection: collectionName}, startTime, "createDocument", span)

	collection, err := d.client.DB.getCollection(tracerCtx, dbName, collectionName)
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
	tracerCtx, span := d.client.addTrace(ctx, "getDocument", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer d.client.sendOperationStats(&QueryLog{Query: "getDocument", Collection: collectionName, ID: documentID}, startTime, "getDocument", span)

	collection, err := d.client.DB.getCollection(tracerCtx, dbName, collectionName)
	if err != nil {
		return err
	}

	_, err = collection.ReadDocument(tracerCtx, documentID, result)

	return err
}

// UpdateDocument updates an existing document in the specified collection.
func (d *Document) UpdateDocument(ctx context.Context, dbName, collectionName, documentID string, document any) error {
	tracerCtx, span := d.client.addTrace(ctx, "updateDocument", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer d.client.sendOperationStats(&QueryLog{Query: "updateDocument", Collection: collectionName,
		ID: documentID}, startTime, "updateDocument", span)

	collection, err := d.client.DB.getCollection(tracerCtx, dbName, collectionName)
	if err != nil {
		return err
	}

	_, err = collection.UpdateDocument(tracerCtx, documentID, document)

	return err
}

// DeleteDocument deletes a document by its ID from the specified collection.
func (d *Document) DeleteDocument(ctx context.Context, dbName, collectionName, documentID string) error {
	tracerCtx, span := d.client.addTrace(ctx, "deleteDocument", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer d.client.sendOperationStats(&QueryLog{Query: "deleteDocument", Collection: collectionName,
		ID: documentID}, startTime, "deleteDocument", span)

	collection, err := d.client.DB.getCollection(tracerCtx, dbName, collectionName)
	if err != nil {
		return err
	}

	_, err = collection.DeleteDocument(tracerCtx, documentID)

	return err
}

// CreateEdgeDocument creates a new edge document between two vertices.
func (d *Document) CreateEdgeDocument(ctx context.Context, dbName, collectionName, from, to string, document any) (string, error) {
	tracerCtx, span := d.client.addTrace(ctx, "createEdgeDocument", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer d.client.sendOperationStats(&QueryLog{Query: "createEdgeDocument", Collection: collectionName}, startTime, "createEdgeDocument", span)

	collection, err := d.client.DB.getCollection(tracerCtx, dbName, collectionName)
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
