# GoFr
<p align="center">
<img align="center" width="300" alt="logo" src="https://github.com/gofr-dev/gofr/assets/44036979/916fe7b1-42fb-4af1-9e0b-4a7a064c243c">
</p>

<div align=center>
<a href="https://pkg.go.dev/gofr.dev"><img src="https://img.shields.io/badge/%F0%9F%93%9A%20godoc-pkg-00ACD7.svg?color=00ACD7&style=flat-square"></a>
<a href="https://gofr.dev/docs"><img src="https://img.shields.io/badge/%F0%9F%92%A1%20gofr-docs-00ACD7.svg?style=flat-square"></a>
<a href="https://codeclimate.com/github/gofr-dev/gofr/maintainability"><img src="https://api.codeclimate.com/v1/badges/58c8d0443a3d08c59c07/maintainability" /></a>
<a href="https://codeclimate.com/github/gofr-dev/gofr/test_coverage"><img src="https://api.codeclimate.com/v1/badges/58c8d0443a3d08c59c07/test_coverage" /></a>
<a href="https://goreportcard.com/report/gofr.dev"><img src="https://goreportcard.com/badge/gofr.dev"></a>
<a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg"></a>

</div>


<br>

Gofr is an opinionated microservice development framework. Listed in [CNCF Landscape](https://landscape.cncf.io/?selected=go-fr).

Visit <a href="https://gofr.dev"/>https://gofr.dev</a> for more details and documentation.

## 🎯 Goal
Even though generic applications can be written using Gofr, our main focus is to simplify the development of microservices.
We will focus ourselves towards deployment in kubernetes and aspire to provide out-of-the-box observability.

[//]: # (## ⚡️ Quick Start Guide)

[//]: # ()
[//]: # (If you already have a go project with go module, you can get gofr by calling: `go get gofr.dev`. Follow the instructions below, if you are starting afresh.)

[//]: # ()
[//]: # (The latest version of go in your system should be installed. If you have not already done that, install it from [here]&#40;https://go.dev/&#41;. This can be tested by opening a terminal and trying `go version`. One should also be familiar with golang syntax. Official golang website has an excellent [tour of go]&#40;https://go.dev/tour/welcome/1&#41; and is highly recommended.)

[//]: # ()
[//]: # (Writing an API service using Gofr is very simple.)

[//]: # (1. In an empty folder, initialise your go module using: `go mod init test-service`. If you intend to push your code to github, it is recommended to name your module like this: `go mod init github.com/{USERNAME}/{REPO}`)

[//]: # (2. Create `main.go` file with following content:)

[//]: # (```go)

[//]: # (package main)

[//]: # ()
[//]: # (import "gofr.dev/pkg/gofr")

[//]: # ()
[//]: # (func main&#40;&#41; {)

[//]: # (    app := gofr.New&#40;&#41;)

[//]: # ()
[//]: # (    app.GET&#40;"/", func&#40;ctx *gofr.Context&#41; &#40;interface{}, error&#41; {)

[//]: # (        return "Hello World!", nil)

[//]: # (    }&#41;)

[//]: # ()
[//]: # (    app.Start&#40;&#41;)

[//]: # (})

[//]: # (```)

[//]: # (3. Get all the dependencies using `go get ./...`. It will download gofr along with every other package it requires.)

[//]: # (4. Start the server: `go run main.go` It will start the server on default port 8000. If this port is already in use, you can override the default port by mentioning an environment variable like this: `HTTP_PORT=9000 go run main.go`)

## 💡 Advantages/Features

1. Simple API syntax
2. REST Standards by default
3. [Configuration management](https://gofr.dev/docs/v1/references/configs)
4. Inbuilt Middlewares
5. [Error Management](https://gofr.dev/docs/v1/references/errors)
6. [gRPC support](https://gofr.dev/docs/v1/advanced-guide/grpc)

## 👍 Contribute
If you want to say thank you and/or support the active development of GoFr:

1. Add a [GitHub Star](https://github.com/gofr-dev/gofr/stargazers) to the project.
2. Write a review or tutorial on [Medium](https://medium.com/), [Dev.to](https://dev.to/) or personal blog.
3. Visit [CONTRIBUTING](CONTRIBUTING.md) for details on submitting patches and the contribution workflow.
