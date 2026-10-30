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

	// elasticsearchSourceField is the key of a hit's stored document.
	elasticsearchSourceField = "_source"
	// elasticsearchMappingType is the key naming a field's type in an index mapping.
	elasticsearchMappingType = "type"

	// elasticsearchMigrationPageSize is how many migration records one search returns. It is far below
	// Elasticsearch's default index.max_result_window (10000), so an index configured with a smaller
	// window still works.
	elasticsearchMigrationPageSize = 1000
	// elasticsearchSearchAfter is the search request key carrying the sort values of the last hit read.
	elasticsearchSearchAfter = "search_after"
)

// getElasticsearchMigrationVersionsQuery builds the query for the first page of applied migration
// versions, highest first. Later pages come from getElasticsearchMigrationVersionsPageQuery.
func getElasticsearchMigrationVersionsQuery() map[string]any {
	return map[string]any{
		"size": elasticsearchMigrationPageSize,
		"sort": []map[string]any{
			{migrationFieldVersion: map[string]any{"order": "desc"}},
		},
		elasticsearchSourceField: []string{migrationFieldVersion},
	}
}

// getElasticsearchMigrationVersionsPageQuery builds the query for the page of versions below lastVersion.
// The version is the document ID, so it is unique and needs no tiebreaker in the sort.
func getElasticsearchMigrationVersionsPageQuery(lastVersion int64) map[string]any {
	query := getElasticsearchMigrationVersionsQuery()
	query[elasticsearchSearchAfter] = []any{lastVersion}

	return query
}

// getElasticsearchMigrationVersions reads every applied migration version, one page at a time, so
// neither the result window nor the size of the index can leave an applied version out.
func getElasticsearchMigrationVersions(c *container.Container) ([]int64, error) {
	var (
		versions []int64
		query    = getElasticsearchMigrationVersionsQuery()
	)

	for {
		result, err := c.Elasticsearch.Search(context.Background(), []string{elasticsearchMigrationIndex}, query)
		if err != nil {
			return nil, err
		}

		page, hitCount := extractMigrationVersions(result)
		versions = append(versions, page...)

		// A short page is the last one. Stopping when no version was read guards against paging forever.
		if hitCount < elasticsearchMigrationPageSize || len(page) == 0 {
			return versions, nil
		}

		query = getElasticsearchMigrationVersionsPageQuery(page[len(page)-1])
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
					migrationFieldVersion: map[string]any{
						elasticsearchMappingType: "long",
					},
					migrationFieldMethod: map[string]any{
						elasticsearchMappingType: "keyword",
					},
					migrationFieldStartTime: map[string]any{
						elasticsearchMappingType: "date",
					},
					migrationFieldDuration: map[string]any{
						elasticsearchMappingType: "long",
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

// getAppliedMigrations retrieves the applied migration versions from Elasticsearch.
func (em elasticsearchMigrator) getAppliedMigrations(c *container.Container) (map[int64]struct{}, error) {
	versions, err := getElasticsearchMigrationVersions(c)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: %w", err)
	}

	c.Debugf("Elasticsearch applied migrations fetched: %v", versions)

	applied, err := em.migrator.getAppliedMigrations(c)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		applied[v] = struct{}{}
	}

	return applied, nil
}

// extractMigrationVersions extracts the migration versions from the Elasticsearch search result,
// along with how many hits the result held.
func extractMigrationVersions(result map[string]any) (versions []int64, hitCount int) {
	hits, ok := result["hits"].(map[string]any)
	if !ok {
		return nil, 0
	}

	hitsList, ok := hits["hits"].([]any)
	if !ok {
		return nil, 0
	}

	versions = make([]int64, 0, len(hitsList))

	for _, h := range hitsList {
		hit, ok := h.(map[string]any)
		if !ok {
			continue
		}

		source, ok := hit[elasticsearchSourceField].(map[string]any)
		if !ok {
			continue
		}

		version, ok := source[migrationFieldVersion].(float64)
		if !ok {
			continue
		}

		versions = append(versions, int64(version))
	}

	return versions, len(hitsList)
}

// beginTransaction starts a new transaction (Elasticsearch doesn't support traditional transactions).
func (em elasticsearchMigrator) beginTransaction(c *container.Container) transactionData {
	return em.migrator.beginTransaction(c)
}

// commitMigration records the migration in the tracking index.
func (em elasticsearchMigrator) commitMigration(c *container.Container, data *transactionData) error {
	if data.UsedDatasources[dsElasticsearch] {
		migrationDoc := map[string]any{
			migrationFieldVersion:   data.MigrationNumber,
			migrationFieldMethod:    migrationMethodUP,
			migrationFieldStartTime: data.StartTime.Format(time.RFC3339),
			migrationFieldDuration:  time.Since(data.StartTime).Milliseconds(),
		}

		// Use the migration number as the document ID for idempotency
		docID := fmt.Sprintf("%d", data.MigrationNumber)

		err := c.Elasticsearch.IndexDocument(context.Background(), elasticsearchMigrationIndex, docID, migrationDoc)
		if err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}

		c.Debugf("Inserted record for migration %v in Elasticsearch gofr_migrations index", data.MigrationNumber)
	}

	return em.migrator.commitMigration(c, data)
}

// rollback is a no-op for Elasticsearch migrations.
func (em elasticsearchMigrator) rollback(c *container.Container, data *transactionData) {
	em.migrator.rollback(c, data)
	c.Fatalf("Migration %v failed.", data.MigrationNumber)
}

func (em elasticsearchMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	return em.migrator.lock(ctx, cancel, c, ownerID)
}

func (em elasticsearchMigrator) unlock(c *container.Container, ownerID string) error {
	return em.migrator.unlock(c, ownerID)
}

func (elasticsearchMigrator) name() string {
	return "Elasticsearch"
}
