package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// IndexDocument indexes (creates or replaces) a single document.
func (c *Client) IndexDocument(ctx context.Context, index, id string, document any) error {
	if strings.TrimSpace(index) == "" {
		return errEmptyIndex
	}

	if strings.TrimSpace(id) == "" {
		return errEmptyDocumentID
	}

	start := time.Now()

	tracedCtx, span := c.addTrace(ctx, "index-document", []string{index}, id)

	body, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("%w: document: %w", errMarshaling, err)
	}

	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: id,
		Body:       bytes.NewReader(body),
		Refresh:    "true",
	}

	res, err := req.Do(tracedCtx, c.client)
	if err != nil {
		return fmt.Errorf("%w: indexing document: %w", errOperation, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("%w: %s", errResponse, res.String())
	}

	c.sendOperationStats(start, fmt.Sprintf("INDEX DOCUMENT %s/%s", index, id), []string{id},
		"", document, span)

	return nil
}

// GetDocument retrieves a document by its ID.
func (c *Client) GetDocument(ctx context.Context, index, id string) (map[string]any, error) {
	if strings.TrimSpace(index) == "" {
		return nil, errEmptyIndex
	}

	if strings.TrimSpace(id) == "" {
		return nil, errEmptyDocumentID
	}

	start := time.Now()

	tracedCtx, span := c.addTrace(ctx, "get-document", []string{index}, id)

	req := esapi.GetRequest{
		Index:      index,
		DocumentID: id,
	}

	res, err := req.Do(tracedCtx, c.client)
	if err != nil {
		return nil, fmt.Errorf("%w: getting document: %w", errOperation, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("%w: %s", errResponse, res.String())
	}

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %w", errParsingResponse, err)
	}

	c.sendOperationStats(start, fmt.Sprintf("GET DOCUMENT %s/%s", index, id),
		[]string{index}, id, nil, span)

	return result, nil
}

// UpdateDocument applies a partial update to an existing document.
func (c *Client) UpdateDocument(ctx context.Context, index, id string, update map[string]any) error {
	if strings.TrimSpace(index) == "" {
		return errEmptyIndex
	}

	if strings.TrimSpace(id) == "" {
		return errEmptyDocumentID
	}

	if len(update) == 0 {
		return errEmptyQuery
	}

	start := time.Now()

	tracedCtx, span := c.addTrace(ctx, "update-document", []string{index}, id)

	body, err := json.Marshal(map[string]any{"doc": update})
	if err != nil {
		return fmt.Errorf("%w: update: %w", errMarshaling, err)
	}

	req := esapi.UpdateRequest{
		Index:      index,
		DocumentID: id,
		Body:       bytes.NewReader(body),
		Refresh:    "true",
	}

	res, err := req.Do(tracedCtx, c.client)
	if err != nil {
		return fmt.Errorf("%w: updating document: %w", errOperation, err)
	}

	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("%w: %s", errResponse, res.String())
	}

	c.sendOperationStats(start, fmt.Sprintf("UPDATE DOCUMENT %s/%s", index, id),
		[]string{index}, id, map[string]any{"doc": update}, span)

	return nil
}

// DeleteDocument removes a document by ID.
func (c *Client) DeleteDocument(ctx context.Context, index, id string) error {
	if strings.TrimSpace(index) == "" {
		return errEmptyIndex
	}

	if strings.TrimSpace(id) == "" {
		return errEmptyDocumentID
	}

	start := time.Now()

	tracedCtx, span := c.addTrace(ctx, "delete-document", []string{index}, id)

	req := esapi.DeleteRequest{
		Index:      index,
		DocumentID: id,
	}

	res, err := req.Do(tracedCtx, c.client)
	if err != nil {
		return fmt.Errorf("%w: deleting document: %w", errOperation, err)
	}

	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("%w: %s", errResponse, res.String())
	}

	c.sendOperationStats(start, fmt.Sprintf("DELETE DOCUMENT %s/%s", index, id),
		[]string{index}, id, nil, span)

	return nil
}
