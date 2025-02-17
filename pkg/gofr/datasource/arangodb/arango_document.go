package arangodb

import (
	"context"
	"errors"
	"time"

	"github.com/arangodb/go-driver/v2/arangodb"
)

var (
	errInvalidEdgeDocumentType = errors.New("document must be a map when creating an edge")
	errMissingEdgeFields       = errors.New("missing '_from' or '_to' field for edge document")
	errInvalidFromField        = errors.New("'_from' field must be a string")
	errInvalidToField          = errors.New("'_to' field must be a string")
)

type Document struct {
	client *Client
}

// CreateDocument creates a new document in the specified collection.
// If the collection is an edge collection, the document must include `_from` and `_to`.
//
// Parameters:
//   - ctx: Request context for tracing and cancellation.
//   - dbName: Name of the database where the document will be created.
//   - collectionName: Name of the collection where the document will be created.
//   - document: The document to be created. For edge collections, it must include `_from` and `_to` fields.
//
// Returns the ID of the created document and an error if the document creation fails.
//
// Example for creating a regular document:
//
//	doc := map[string]any{
//	   "name": "Alice",
//	   "age": 30,
//	}
//
//	id, err := client.CreateDocument(ctx, "myDB", "users", doc)
//
// Example for creating an edge document:
//
//	edgeDoc := map[string]any{
//	   "_from": "users/123",
//	   "_to": "orders/456",
//	   "relation": "purchased",
//	}
//
// id, err := client.CreateDocument(ctx, "myDB", "edges", edgeDoc).
func (d *Document) CreateDocument(ctx context.Context, dbName, collectionName string, document any) (string, error) {
	collection, tracerCtx, err := executeCollectionOperation(ctx, *d, dbName, collectionName,
		"createDocument", "")
	if err != nil {
		return "", err
	}

	var isEdge bool

	// Check if the collection is an edge collection
	isEdge, err = d.isEdgeCollection(ctx, dbName, collectionName)
	if err != nil {
		return "", err
	}

	// Validate edge document if needed
	if isEdge {
		err = validateEdgeDocument(document)
		if err != nil {
			return "", err
		}
	}

	// Create the document in ArangoDB
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

// isEdgeCollection checks if the given collection is an edge collection.
func (d *Document) isEdgeCollection(ctx context.Context, dbName, collectionName string) (bool, error) {
	collection, err := d.client.getCollection(ctx, dbName, collectionName)
	if err != nil {
		return false, err
	}

	properties, err := collection.Properties(ctx)
	if err != nil {
		return false, err
	}

	// ArangoDB type: 3 = Edge Collection, 2 = Document Collection
	return properties.Type == arangoEdgeCollectionType, nil
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

	collection, err := d.client.getCollection(tracerCtx, dbName, collectionName)
	if err != nil {
		return nil, nil, err
	}

	return collection, tracerCtx, nil
}

// validateEdgeDocument ensures the document contains valid `_from` and `_to` fields when creating an edge.
func validateEdgeDocument(document any) error {
	docMap, ok := document.(map[string]any)
	if !ok {
		return errInvalidEdgeDocumentType
	}

	from, fromExists := docMap["_from"]
	to, toExists := docMap["_to"]

	if !fromExists || !toExists {
		return errMissingEdgeFields
	}

	// Ensure `_from` and `_to` are strings
	if _, ok := from.(string); !ok {
		return errInvalidFromField
	}

	if _, ok := to.(string); !ok {
		return errInvalidToField
	}

	return nil
}
