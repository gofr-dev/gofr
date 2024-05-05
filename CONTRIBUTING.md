## Contribution Guidelines
* Minor changes can be done directly by editing code on github. Github automatically creates a temporary branch and
  files a PR. This is only suitable for really small changes like: spelling fixes, variable name changes or error string
  change etc. For larger commits, following steps are recommended.
* (Optional) If you want to discuss your implementation with the users of Gofr, use the github discussions of this repo.
* Configure your editor to use goimport and golangci-lint on file changes. Any code which is not formatted using these
  tools, will fail on the pipeline.
* All code contributions should have associated tests and all new line additions should be covered in those testcases.
  No PR should ever decrease the overall code coverage.
* Once your code changes are done along with the testcases, submit a PR to development branch. Please note that all PRs
  are merged from feature branches to development first.
* All PRs need to be reviewed by at least 2 Gofr developers. They might reach out to you for any clarfication.
* Thank you for your contribution. :)

### GoFr Testing Policy:

Testing is a crucial aspect of software development, and adherence to these guidelines ensures the stability, reliability, and maintainability of the GoFr codebase.

### Guidelines

1.  **Test Types:**

    -   Write unit tests for every new function or method.
    -   Include integration tests for any major feature added.


2. **Test Coverage:**

-   No new code should decrease the existing code coverage for the packages and files.
> The `code-climate` coverage check will not pass if there is any decrease in the test-coverage before and after any new PR is submitted.



3. **Naming Conventions:**

-   Prefix unit test functions with `Test`.
-   Use clear and descriptive names.
```go
func TestFunctionName(t *testing.T) { 
// Test logic 
}
```



4. **Table-Driven Tests:**

-   Consider using table-driven tests for testing multiple scenarios.

**NOTE:**
```go
Some services will be required to pass the entire test suite. We recommend using docker for running those services.

docker run --name gofr-mysql -e MYSQL_ROOT_PASSWORD=password -p 2001:3306 -d mysql:8.0.30
docker run --name gofr-redis -p 2002:6379 -d redis:7.0.5
docker run --name gofr-zipkin -d -p 2005:9411 openzipkin/zipkin:2
docker run --name gofr-pgsql -d -e POSTGRES_DB=customers -e POSTGRES_PASSWORD=root123 -p 2006:5432 postgres:15.1
docker run --name gofr-mssql -d -e 'ACCEPT_EULA=Y' -e 'SA_PASSWORD=reallyStrongPwd123' -p 2007:1433 mcr.microsoft.com/azure-sql-edge
docker run --name zookeeper -e ZOOKEEPER_CLIENT_PORT=2181 -e ZOOKEEPER_TICK_TIME=2000 confluentinc/cp-zookeeper:7.0.1
docker run --name broker -p 9092:9092 --link zookeeper -e KAFKA_BROKER_ID=1 \
-e KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181 \
-e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=PLAINTEXT:PLAINTEXT,PLAINTEXT_INTERNAL:PLAINTEXT \
-e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092,PLAINTEXT_INTERNAL://broker:29092 \
-e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
-e KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1 \
-e KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1 \
confluentinc/cp-kafka:7.0.1

Please note that the recommended local port for the services are different than the actual ports. This is done to avoid conflict with the local installation on developer machines. This method also allows a developer to work on multiple projects which uses the same services but bound on different ports. One can choose to change the port for these services. Just remember to add the same in configs/.local.env, if you decide to do that.
```

### Coding Guidelines
* Use only what is given to you as part of function parameter or receiver. No globals. Inject all dependencies including
  DB, Logger etc.
* No magic. So, no init. In a large project, it becomes difficult to track which package is doing what at the
  initialisation step.
* Exported functions must have an associated goDoc.
* Sensitive data(username, password, keys) should not be pushed. Always use environment variables.
* Take interfaces and return concrete types.
    - Lean interfaces - take 'exactly' what you need, not more. Onus of interface definition is on the package who is
      using it. so, it should be as lean as possible. This makes it easier to test.
    - Be careful of type assertions in this context. If you take an interface and type assert to a type - then its
      similar to taking concrete type.
* Uses of context:
    - We should use context as a first parameter.
    - Can not use string as a key for the context. Define your own type and provide context accessor method to avoid
      conflict.
* External Library uses:
    - A little copying is better than a little dependency.
    - All external dependencies should go through the same careful consideration, we would have done to our own written
      code. We need to test the functionality we are going to use from an external library, as sometimes library
      implementation may change.
    - All dependencies must be abstracted as an interface. This will make it easier to switch libraries at later point
      of time.
* Version tagging as per Semantic versioning (https://semver.org/)

### Documentation
* After adding or modifying code existing code, update the documentation too - [development/docs](https://github.com/gofr-dev/gofr/tree/development/docs).
* If needed, update or add proper code examples for your changes.
* Make sure you don't break existing links and refernces.
* The [gofr.dev documentation]([url](https://gofr.dev/docs)) site is updated upon push to `/docs` path in the repo. Verify your changes are live after PullRequest is merged.
