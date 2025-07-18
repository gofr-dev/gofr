package migration

import (
	"context"
	"fmt"
	"time"

	"gofr.dev/pkg/gofr/container"
)

// elasticsearchDS is the adapter struct that implements migration operations for Elasticsearch.
type elasticsearchDS struct {
	client Elasticsearch
}

// elasticsearchMigrator struct implements the migrator interface for Elasticsearch.
type elasticsearchMigrator struct {
	elasticsearchDS
	migrator
}

const (
	// elasticsearchMigrationIndex is the index used to track migrations.
	elasticsearchMigrationIndex = "gofr_migrations"
)

// getLastElasticsearchMigrationQuery fetches the most recent migration version.
func getLastElasticsearchMigrationQuery() map[string]any {
	return map[string]any{
		"size": 1,
		"sort": []map[string]any{
			{"version": map[string]any{"order": "desc"}},
		},
		"_source": []string{"version"},
	}
}

// apply creates a new elasticsearchMigrator.
func (ds elasticsearchDS) apply(m migrator) migrator {
	return elasticsearchMigrator{
		elasticsearchDS: ds,
		migrator:        m,
	}
}

// checkAndCreateMigrationTable creates the migration tracking index if it doesn't exist.
func (em elasticsearchMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	// Check if the migration index exists
	query := map[string]any{
		"query": map[string]any{
			"match_all": map[string]any{},
		},
		"size": 0,
	}

	_, err := c.Elasticsearch.Search(context.Background(), []string{elasticsearchMigrationIndex}, query)
	if err != nil {
		c.Debug("Migration index might already exist:", err)
		// Index doesn't exist, create it
		settings := map[string]any{
			"settings": map[string]any{
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
			"mappings": map[string]any{
				"properties": map[string]any{
					"version": map[string]any{
						"type": "long",
					},
					"method": map[string]any{
						"type": "keyword",
					},
					"start_time": map[string]any{
						"type": "date",
					},
					"duration": map[string]any{
						"type": "long",
					},
				},
			},
		}

		err = c.Elasticsearch.CreateIndex(context.Background(), elasticsearchMigrationIndex, settings)
		if err != nil {
			return fmt.Errorf("failed to create migration index: %w", err)
		}

		c.Debugf("Created Elasticsearch migration index: %s", elasticsearchMigrationIndex)
	}

	return em.migrator.checkAndCreateMigrationTable(c)
}

// getLastMigration retrieves the latest migration version from Elasticsearch.
func (em elasticsearchMigrator) getLastMigration(c *container.Container) int64 {
	var lastMigration int64

	result, err := c.Elasticsearch.Search(context.Background(), []string{elasticsearchMigrationIndex}, getLastElasticsearchMigrationQuery())
	if err != nil {
		c.Errorf("Failed to fetch migrations from Elasticsearch: %v", err)
		return 0
	}

	lastMigration = extractLastMigrationVersion(result)
	c.Debugf("Elasticsearch last migration fetched value is: %v", lastMigration)

	lm2 := em.migrator.getLastMigration(c)
	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

// extractLastMigrationVersion extracts the latest migration version from the Elasticsearch search result.
func extractLastMigrationVersion(result map[string]any) int64 {
	hits, ok := result["hits"].(map[string]any)
	if !ok {
		return 0
	}

	hitsList, ok := hits["hits"].([]any)
	if !ok || len(hitsList) == 0 {
		return 0
	}

	firstHit, ok := hitsList[0].(map[string]any)
	if !ok {
		return 0
	}

	source, ok := firstHit["_source"].(map[string]any)
	if !ok {
		return 0
	}

	version, ok := source["version"].(float64)
	if !ok {
		return 0
	}

	return int64(version)
}

// beginTransaction starts a new transaction (Elasticsearch doesn't support traditional transactions).
func (em elasticsearchMigrator) beginTransaction(c *container.Container) transactionData {
	return em.migrator.beginTransaction(c)
}

// commitMigration records the migration in the tracking index.
func (em elasticsearchMigrator) commitMigration(c *container.Container, data transactionData) error {
	migrationDoc := map[string]any{
		"version":    data.MigrationNumber,
		"method":     "UP",
		"start_time": data.StartTime.Format(time.RFC3339),
		"duration":   time.Since(data.StartTime).Milliseconds(),
	}

	// Use the migration number as the document ID for idempotency
	docID := fmt.Sprintf("%d", data.MigrationNumber)

	err := c.Elasticsearch.IndexDocument(context.Background(), elasticsearchMigrationIndex, docID, migrationDoc)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	c.Debugf("Inserted record for migration %v in Elasticsearch gofr_migrations index", data.MigrationNumber)

	return em.migrator.commitMigration(c, data)
}

// rollback is a no-op for Elasticsearch migrations.
func (em elasticsearchMigrator) rollback(c *container.Container, data transactionData) {
	em.migrator.rollback(c, data)
	c.Fatalf("Migration %v failed.", data.MigrationNumber)
}
