# GoFr
<p align="center">
<img align="center" width="300" alt="logo" src="https://github.com/gofr-dev/gofr/assets/44036979/916fe7b1-42fb-4af1-9e0b-4a7a064c243c">
</p>
<br /><br /><br />

<a href="https://codeclimate.com/github/gofr-dev/gofr/maintainability"><img src="https://api.codeclimate.com/v1/badges/58c8d0443a3d08c59c07/maintainability" /></a>
<a href="https://codeclimate.com/github/gofr-dev/gofr/test_coverage"><img src="https://api.codeclimate.com/v1/badges/58c8d0443a3d08c59c07/test_coverage" /></a>
[![Go Report Card](https://goreportcard.com/badge/gofr.dev)](https://goreportcard.com/report/gofr.dev)
<a href="https://pkg.go.dev/gofr.dev/pkg/gofr"><img src="https://pkg.go.dev/badge/gofr.dev.svg" alt="Go Reference"></a>
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Gofr is an opinionated microservice development framework. Listed in [CNCF Landscape](https://landscape.cncf.io/?selected=go-fr)

Visit <a href="https://gofr.dev"/>https://gofr.dev</a> for more details and documentation. 

## Goal
Even though generic applications can be written using Gofr, our main focus is to simplify the development of microservices. 
We will focus ourselves towards deployment in kubernetes and aspire to provide out-of-the-box observability. 

## Advantages

1. Simple API syntax
2. REST Standards by default
3. Battle Tested at Enterprise Scale
4. Configuration management
5. Database state management using migrations
6. Inbuilt Middlewares along with support for custom middlewares
7. Error Management
8. Inbuilt Datastore, File System, Pub/Sub Support
9. gRPC support 
10. Chained timeout control
11. Serve Static Files
12. Websocket Support
13. Support for Surge Protection
14. Inbuilt Traces, Metrics and Logs

## Quick Start Guide

If you already have a go project with go module, you can get gofr by calling: `go get gofr.dev`. Follow the instructions below, if you are starting afresh. 

The latest version of go in your system should be installed. If you have not already done that, install it from [here](https://go.dev/). This can be tested by opening a terminal and trying `go version`. One should also be familiar with golang syntax. Official golang website has an excellent [tour of go](https://go.dev/tour/welcome/1) and is highly recommended.  

Writing an API service using Gofr is very simple. 
1. In an empty folder, initialise your go module using: `go mod init test-service`. If you intend to push your code to github, it is recommended to name your module like this: `go mod init github.com/{USERNAME}/{REPO}`
2. Create `main.go` file with following content: 
```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    app := gofr.New()

    app.GET("/", func(ctx *gofr.Context) (interface{}, error) {
        return "Hello World!", nil
    })

    app.Start()
}
```
3. Get all the dependencies using `go get ./...`. It will download gofr along with every other package it requires.
4. Start the server: `go run main.go` It will start the server on default port 8000. If this port is already in use, you can override the default port by mentioning an environment variable like this: `HTTP_PORT=9000 go run main.go`
