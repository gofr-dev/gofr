# gofr
![Build Status](https://github.com/vikash/gofr/workflows/Go/badge.svg)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://raw.githubusercontent.com/jmoiron/sqlx/master/LICENSE)
[![Maintainability](https://api.codeclimate.com/v1/badges/b23df337f31dedbfa918/maintainability)](https://codeclimate.com/github/vikash/gofr/maintainability)
[![Coverage Status](https://coveralls.io/repos/github/vikash/gofr/badge.svg?branch=main)](https://coveralls.io/github/vikash/gofr?branch=main)

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


### Development Notes
To run the tests, mysql and redis needs to run on the default ports 3306 and 6379 respectively. Following
docker commands can be used: 
* `docker run --name gofr-mysql -e MYSQL_ROOT_PASSWORD=password -p3306:3306 mysql:latest`
* `docker run --name gofr-redis -p6379:6379 redis:latest`