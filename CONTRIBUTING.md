## Contribution Guidelines
* Minor changes can be done directly by editing code on GitHub. GitHub automatically creates a temporary branch and
  files a PR. This is only suitable for really small changes like: spelling fixes, variable name changes or error string
  change etc. For larger commits, following steps are recommended.
* (Optional) If you want to discuss your implementation with the users of GoFr, use the GitHub discussions of this repo.
* Configure your editor to use goimport and golangci-lint on file changes. Any code which is not formatted using these
  tools, will fail on the pipeline.
* All code contributions should have associated tests and all new line additions should be covered in those testcases.
  No PR should ever decrease the overall code coverage.
* Once your code changes are done along with the testcases, submit a PR to development branch. Please note that all PRs
  are merged from feature branches to development first.
* PR should be raised only when development is complete and the code is ready for review. This approach helps reduce the number of open pull requests and facilitates a more efficient review process for the team.
* All PRs need to be reviewed by at least 2 GoFr developers. They might reach out to you for any clarification.
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

docker run --name mongodb -d -p 27017:27017 -e MONGO_INITDB_ROOT_USERNAME=user -e MONGO_INITDB_ROOT_PASSWORD=password mongodb/mongodb-community-server:latest
docker run -d -p 21:21 -p 21000-21010:21000-21010 -e USERS='user|password' delfer/alpine-ftp-server
// the docker image is relatively unstable. Alternatively, refer to official guide of OpenTSDB to locally setup OpenTSDB env.
// http://opentsdb.net/docs/build/html/installation.html#id1
docker run -d --name gofr-opentsdb -p 4242:4242 petergrace/opentsdb-docker:latest
docker run --name gofr-mysql -e MYSQL_ROOT_PASSWORD=password -e MYSQL_DATABASE=test -p 2001:3306 -d mysql:8.0.30
docker run --name gofr-redis -p 2002:6379 -d redis:7.0.5
docker run --name gofr-zipkin -d -p 2005:9411 openzipkin/zipkin:2
docker run --rm -it -p 4566:4566 -p 4510-4559:4510-4559 localstack/localstack
docker run --name cassandra-node -d -p 9042:9042 -v cassandra_data:/var/lib/cassandra cassandra:latest
docker run --name gofr-pgsql -d -e POSTGRES_DB=customers -e POSTGRES_PASSWORD=root123 -p 2006:5432 postgres:15.1
docker run --name gofr-mssql -d -e 'ACCEPT_EULA=Y' -e 'SA_PASSWORD=reallyStrongPwd123' -p 2007:1433 mcr.microsoft.com/azure-sql-edge
docker run --name kafka-1 -p 9092:9092 \
 -e KAFKA_ENABLE_KRAFT=yes \
-e KAFKA_CFG_PROCESS_ROLES=broker,controller \
-e KAFKA_CFG_CONTROLLER_LISTENER_NAMES=CONTROLLER \
-e KAFKA_CFG_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093 \
-e KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT \
-e KAFKA_CFG_ADVERTISED_LISTENERS=PLAINTEXT://127.0.0.1:9092 \
-e KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE=true \
-e KAFKA_BROKER_ID=1 \
-e KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=1@127.0.0.1:9093 \
-e ALLOW_PLAINTEXT_LISTENER=yes \
-e KAFKA_CFG_NODE_ID=1 \
-v kafka_data:/bitnami \
bitnami/kafka:3.4 




Please note that the recommended local port for the services are different than the actual ports. This is done to avoid conflict with the local installation on developer machines. This method also allows a developer to work on multiple projects which uses the same services but bound on different ports. One can choose to change the port for these services. Just remember to add the same in configs/.local.env, if you decide to do that.
```

### Coding Guidelines
* Use only what is given to you as part of function parameter or receiver. No globals. Inject all dependencies including
  DB, Logger etc.
* No magic. So, no init. In a large project, it becomes difficult to track which package is doing what at the
  initialization step.
* Exported functions must have an associated goDoc.
* Sensitive data(username, password, keys) should not be pushed. Always use environment variables.
* Take interfaces and return concrete types.
    - Lean interfaces - take 'exactly' what you need, not more. Onus of interface definition is on the package who is
      using it. so, it should be as lean as possible. This makes it easier to test.
    - Be careful of type assertions in this context. If you take an interface and type assert to a type - then it's
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
* When you consider a new documentation page is needed, start by adding a new file and writing your new documentation. Then - add a reference to it in [navigation.js](https://gofr.dev/docs/navigation.js).
* If needed, update or add proper code examples for your changes.
* In case images are needed, add it to [docs/public](https://gofr.dev/docs/public) folder.
* Make sure you don't break existing links and references.
* Maintain Markdown standards, you can read more [here](https://www.markdownguide.org/basic-syntax/), this includes:
    - Headings (`#`, `##`, etc.) should be placed in order.
    - Use trailing white space or the <br> HTML tag at the end of the line.
    - Use "`" sign to add single line code and "```" to add multi-line code block.
    - Use relative references to images (in `public` folder as mentioned above.)
* The [gofr.dev documentation]([url](https://gofr.dev/docs)) site is updated upon push to `/docs` path in the repo. Verify your changes are live after next GoFr version.
