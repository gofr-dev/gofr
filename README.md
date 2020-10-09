# gofr
Mini GoLang framework for writing http api and command line tools.

### What all is (planned to be) supported in Gofr?
* API server and Cmd creation
* SQL Database
* Redis
* Open telemetry for tracing/metrics
    * Trace all incoming requests
    * All outbound HTTP or gRPC requests
    * All Postgres queries
    * All Redis commands
    * Export Spans to GCP Cloud trace based on config
* Logs to go to stdout and stderr
* Configurations by Environment
