# gofr
![Build Status](https://github.com/vikash/gofr/workflows/Go/badge.svg)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://raw.githubusercontent.com/jmoiron/sqlx/master/LICENSE)
[![Maintainability](https://api.codeclimate.com/v1/badges/b23df337f31dedbfa918/maintainability)](https://codeclimate.com/github/vikash/gofr/maintainability)
[![Test Coverage](https://api.codeclimate.com/v1/badges/b23df337f31dedbfa918/test_coverage)](https://codeclimate.com/github/vikash/gofr/test_coverage)

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
