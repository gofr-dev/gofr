# Elasticsearch Migration Support in GoFr

## Motivation

Elasticsearch is a powerful search and analytics engine, but managing its schema changes (index creation, mapping updates, analyzers, etc.) is not as straightforward as with SQL databases. These changes must be made via REST API calls and require careful version control to ensure consistency across environments.

GoFr's Elasticsearch migrator brings versioned, idempotent, and trackable migrations to your Elasticsearch clusters, just like you expect for SQL databases.

## How It Works

- **REST API Driven**: All migration operations (index creation, mapping updates, document indexing) are executed via Elasticsearch's REST API.
- **State Tracking**: Migration state is tracked in a dedicated `gofr_migrations` index in your Elasticsearch cluster.
- **Idempotency**: Each migration is versioned and only applied once. Re-running migrations is safe.
- **Integration**: The migrator plugs into GoFr's migration system, so you can manage all your data sources (SQL, NoSQL, Elasticsearch, etc.) in a unified way.

## Key Features

- Create/delete indices
- Update mappings and analyzers
- Index or update documents
- Track migration state and history
- Ensure safe, repeatable migrations

## Getting Started

### 1. Add GoFr's Elasticsearch Driver

```bash
go get gofr.dev/pkg/gofr/datasource/elasticsearch@latest
```

### 2. Configure Elasticsearch in Your App

```go
import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/datasource/elasticsearch"
)

func main() {
    app := gofr.New()
    es := elasticsearch.New(elasticsearch.Config{
        Addresses: []string{"http://localhost:9200"},
        Username:  "elastic",
        Password:  "changeme",
    })
    app.AddElasticsearch(es)
    // ...
}
```

### 3. Write Migrations

Each migration is a function that receives a `migration.Datasource` and can use the `Elasticsearch` interface:

```go
import (
    "context"
    "gofr.dev/pkg/gofr/migration"
)

migrations := map[int64]migration.Migrate{
    1: {
        UP: func(d migration.Datasource) error {
            settings := map[string]any{
                "settings": map[string]any{
                    "number_of_shards": 1,
                    "number_of_replicas": 0,
                },
                "mappings": map[string]any{
                    "properties": map[string]any{
                        "title": map[string]any{"type": "text"},
                    },
                },
            }
            return d.Elasticsearch.CreateIndex(context.Background(), "articles", settings)
        },
    },
    2: {
        UP: func(d migration.Datasource) error {
            doc := map[string]any{"title": "First Article"}
            return d.Elasticsearch.IndexDocument(context.Background(), "articles", "1", doc)
        },
    },
}
```

### 4. Run Migrations

```go
app.Migrate(migrations)
```

### 5. Check Migration Status

You can query the `gofr_migrations` index directly, or add an endpoint:

```go
app.GET("/migrations/status", func(c *gofr.Context) (any, error) {
    query := map[string]any{
        "query": map[string]any{"match_all": map[string]any{}},
        "sort": []map[string]any{{"version": map[string]any{"order": "desc"}}},
    }
    return c.Elasticsearch.Search(context.Background(), []string{"gofr_migrations"}, query)
})
```

## Best Practices

- **Use version numbers**: Always increment migration versions. Use timestamps or sequential numbers.
- **Idempotency**: Write migrations so they can be safely re-run (e.g., use `PUT` for index creation, check existence before creating).
- **Track all changes**: Use migrations for any change to indices, mappings, or analyzers.
- **Test locally**: Always test migrations on a local or staging cluster before production.
- **Backup**: Take regular snapshots of your cluster before running destructive migrations.

## Example: Bulk Operations

```go
3: {
    UP: func(d migration.Datasource) error {
        ops := []map[string]any{
            {"index": map[string]any{"_index": "articles", "_id": "2"}},
            {"title": "Second Article"},
        }
        _, err := d.Elasticsearch.Bulk(context.Background(), ops)
        return err
    },
},
```

## Troubleshooting

- **Migration not applied?**
  - Check the `gofr_migrations` index for the latest version.
  - Ensure your migration version is greater than the last applied.
- **Index already exists?**
  - Use `PUT` for idempotent index creation, or check for existence before creating.
- **Mapping errors?**
  - Review Elasticsearch mapping update rules; some changes require reindexing.

## FAQ

**Q: Can I use this for mapping updates?**
A: Yes! Use the `PutMapping` API via a custom REST call if needed.

**Q: What if a migration fails?**
A: The migrator will stop and log the error. Fix the migration and re-run; previous successful migrations are not re-applied.

**Q: How do I rollback?**
A: Rollback is manual. Write a new migration to reverse changes if needed.

## Reference

- [GoFr Elasticsearch Driver](https://pkg.go.dev/gofr.dev/pkg/gofr/datasource/elasticsearch)
- [Elasticsearch REST API Docs](https://www.elastic.co/guide/en/elasticsearch/reference/current/rest-apis.html)
- [GoFr Migration System](https://gofr.dev/docs/advanced-guide/migrations/)

## See Also

- [Example: Using Elasticsearch Migrations](../../../examples/using-elasticsearch-migrations/README.md) 