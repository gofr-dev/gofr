# Supabase Migration Test Example

This example demonstrates how to use GoFr with Supabase migrations. It includes both a working example and comprehensive tests to verify that Supabase migration support works correctly.

## What This Example Tests

1. **Configuration Loading**: Verifies that GoFr can load Supabase configuration correctly
2. **Migration Structure**: Tests that migration functions are properly structured
3. **Dialect Handling**: Ensures that the `supabase` dialect is handled correctly
4. **Migration Execution Flow**: Tests the complete migration execution process

## Files Included

- `main.go`: A working example that demonstrates Supabase migrations
- `main_test.go`: Comprehensive tests for Supabase migration functionality
- `configs/config.yaml`: Configuration file with mock Supabase values
- `README.md`: This documentation file

## How to Run the Tests

### 1. Run All Tests
```bash
cd examples/supabase-migration-test
go test -v
```

### 2. Run Specific Tests
```bash
# Test configuration loading
go test -v -run TestSupabaseMigrationConfiguration

# Test migration structure
go test -v -run TestSupabaseMigrationStructure

# Test dialect handling
go test -v -run TestSupabaseDialectHandling
```

### 3. Run the Example (with real Supabase)
```bash
# Set your actual Supabase credentials
export DB_DIALECT=supabase
export DB_HOST=db.your-project.supabase.co
export DB_USER=postgres
export DB_PASSWORD=your-password
export DB_NAME=postgres
export DB_PORT=5432
export DB_SSL_MODE=require
export SUPABASE_PROJECT_REF=your-project
export SUPABASE_CONNECTION_TYPE=direct
export SUPABASE_REGION=your-region

# Run the example
go run main.go
```

## Test Results

When you run the tests, you should see output like:

```
=== RUN   TestSupabaseMigrationConfiguration
--- PASS: TestSupabaseMigrationConfiguration (0.00s)
=== RUN   TestSupabaseMigrationStructure
--- PASS: TestSupabaseMigrationStructure (0.00s)
=== RUN   TestSupabaseMigrationMap
--- PASS: TestSupabaseMigrationMap (0.00s)
=== RUN   TestSupabaseDialectHandling
--- PASS: TestSupabaseDialectHandling (0.00s)
=== RUN   TestMigrationExecutionFlow
--- PASS: TestMigrationExecutionFlow (0.00s)
PASS
ok      gofr.dev/examples/supabase-migration-test    0.005s
```

## What the Tests Verify

### TestSupabaseMigrationConfiguration
-  GoFr app can be created with Supabase configuration
-  Environment variables are loaded correctly
-  Supabase-specific configuration is recognized

### TestSupabaseMigrationStructure
-  Migration functions are properly structured
-  SQL syntax is valid for PostgreSQL/Supabase
-  Migration objects are created correctly

### TestSupabaseMigrationMap
-  Multiple migrations can be organized in a map
-  Migration versioning works correctly
-  Complex migration structures are supported

### TestSupabaseDialectHandling
-  `supabase` dialect is recognized
-  PostgreSQL-style bind variables are used
-  Dialect-specific logic works correctly

### TestMigrationExecutionFlow
-  Complete migration flow can be set up
-  App configuration works with migrations
-  Migration definitions are valid

## Key Features Demonstrated

1. **Supabase Dialect Support**: The `supabase` dialect is now fully supported
2. **PostgreSQL Compatibility**: Supabase uses PostgreSQL syntax and features
3. **Migration Management**: Full migration lifecycle management
4. **Configuration Management**: Environment-based configuration loading
5. **Error Handling**: Proper error handling and logging

## Integration with Real Supabase

To test with a real Supabase instance:

1. Create a Supabase project at https://supabase.com
2. Get your connection details from the project settings
3. Update the environment variables with your real credentials
4. Run the example: `go run main.go`

The migration will create a test table in your Supabase database, demonstrating that the integration works correctly.
