# Database Migrations

GoFr provides a robust migration system that allows you to version and manage database schema changes across different datasources. Migrations help you evolve your database schema in a controlled, repeatable way while maintaining data integrity.

## Supported Datasources for Migrations

GoFr supports migrations for the following datasources:

- **SQL Databases** (MySQL, PostgreSQL, SQLite, CockroachDB)
- **NoSQL Databases** (MongoDB, Cassandra, ClickHouse, DGraph, SurrealDB, ArangoDB)
- **Search Engines** (Elasticsearch)
- **Key-Value Stores** (Redis)

## How Migrations Work

1. **Migration Files**: Each migration is identified by a unique timestamp-based ID
2. **Migration Tracking**: GoFr tracks applied migrations in a dedicated table/index
3. **Sequential Execution**: Migrations are executed in chronological order
4. **Rollback Support**: Some datasources support rollback operations
5. **Idempotency**: Migrations can be run multiple times safely

## Migration Structure

Each migration consists of:

```go
migrationsMap := map[int64]migration.Migrate{
    1640995200: { // Unix timestamp
        UP: func(d migration.Datasource) error {
            // Forward migration logic
            return nil
        },
        DOWN: func(d migration.Datasource) error {
            // Rollback logic (optional)
            return nil
        },
    },
}
```

## Getting Started

To use migrations in your GoFr application:

1. **Define your migrations** using the migration map structure
2. **Register migrations** with your GoFr app using `app.Migrate(migrationsMap)`
3. **Run your application** - migrations will be executed automatically on startup

## Datasource-Specific Migration Guides

- [Elasticsearch Migrations](./elasticsearch/) - Index management, mappings, and data seeding
- SQL Migrations - Table creation, schema changes, and data transformations
- MongoDB Migrations - Collection management and document transformations
- Redis Migrations - Key management and data structure changes

## Best Practices

### Migration Naming
- Use descriptive names that explain what the migration does
- Include the timestamp for proper ordering
- Example: `1640995200_create_users_index`

### Migration Content
- Keep migrations small and focused
- Test migrations thoroughly before deployment
- Always provide rollback logic when possible
- Use transactions where supported

### Version Control
- Store migration files in version control
- Never modify existing migrations once they're deployed
- Create new migrations for schema changes

### Testing
- Test migrations on a copy of production data
- Verify both UP and DOWN operations
- Check performance impact on large datasets

## Migration States

GoFr tracks migration states:

- **Pending**: Migration not yet executed
- **Applied**: Migration successfully executed
- **Failed**: Migration execution failed
- **Rolled Back**: Migration was reversed (if supported)

## Error Handling

When a migration fails:

1. **Automatic Rollback**: GoFr attempts to rollback the failed migration
2. **Error Logging**: Detailed error information is logged
3. **Application Halt**: The application stops to prevent data corruption
4. **Manual Intervention**: Review and fix the migration before restarting

## Monitoring and Observability

GoFr provides comprehensive observability for migrations:

- **Logs**: Detailed execution logs with timing information
- **Metrics**: Migration execution time and success/failure rates
- **Traces**: Distributed tracing for migration operations
- **Health Checks**: Migration status in health endpoints

## Advanced Features

### Conditional Migrations
Execute migrations based on conditions:

```go
UP: func(d migration.Datasource) error {
    // Check if index exists before creating
    if !indexExists(d.Elasticsearch, "users") {
        return d.Elasticsearch.CreateIndex(ctx, "users", settings)
    }
    return nil
},
```

### Data Migrations
Migrate data along with schema:

```go
UP: func(d migration.Datasource) error {
    // Create index
    err := d.Elasticsearch.CreateIndex(ctx, "products", settings)
    if err != nil {
        return err
    }
    
    // Seed initial data
    return seedProductData(d.Elasticsearch)
},
```

### Bulk Operations
Use bulk operations for efficient data migrations:

```go
UP: func(d migration.Datasource) error {
    operations := []map[string]any{
        {"index": {"_index": "products", "_id": "1"}},
        {"name": "Product 1", "price": 19.99},
        {"index": {"_index": "products", "_id": "2"}},
        {"name": "Product 2", "price": 29.99},
    }
    
    _, err := d.Elasticsearch.Bulk(ctx, operations)
    return err
},
```

## Troubleshooting

### Common Issues

1. **Migration Order**: Ensure migrations are numbered correctly
2. **Dependencies**: Check that required indices/tables exist
3. **Permissions**: Verify database permissions for schema changes
4. **Resource Limits**: Consider memory and time limits for large migrations

### Recovery Strategies

1. **Manual Rollback**: Use DOWN migrations to revert changes
2. **Data Backup**: Always backup before running migrations
3. **Incremental Approach**: Break large migrations into smaller steps
4. **Testing Environment**: Test migrations in staging first