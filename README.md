<h1 style="text-align: center; font-size: 100px; font-weight: 500;">
    <i>Go</i>Fr
</h1>

<p align="center">
<img align="center" width="300" alt="logo" src="https://github.com/gofr-dev/gofr/assets/44036979/916fe7b1-42fb-4af1-9e0b-4a7a064c243c">
</p>

<h2 align="center" style="font-size: 28px;"><b>GoFr: An Opinionated Microservice Development Framework</b></h2>

<div align="center">
<a href="https://pkg.go.dev/gofr.dev"><img src="https://img.shields.io/badge/GoDoc-Read%20Documentation-blue?style=for-the-badge" alt="godoc"></a>
<a href="https://gofr.dev/docs"><img src="https://img.shields.io/badge/GoFr-Docs-orange?style=for-the-badge" alt="gofr-docs"></a>
<a href="https://codeclimate.com/github/gofr-dev/gofr/maintainability"><img src="https://img.shields.io/codeclimate/maintainability/gofr-dev/gofr?style=for-the-badge" alt="maintainability"></a>
<a href="https://codeclimate.com/github/gofr-dev/gofr/test_coverage"><img src="https://img.shields.io/codeclimate/coverage/gofr-dev/gofr?style=for-the-badge" alt="test-coverage"></a>
<a href="https://goreportcard.com/report/gofr.dev"><img src="https://goreportcard.com/badge/gofr.dev?style=for-the-badge" alt="Go Report Card"></a>
<a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache_2.0-blue?style=for-the-badge" alt="Apache 2.0 License"></a>
<a href="https://discord.gg/wsaSkQTdgq"><img src="https://img.shields.io/badge/discord-join-us?style=for-the-badge&logo=discord&color=7289DA" alt="discord" /></a>
<a href="https://gurubase.io/g/gofr"><img src="https://img.shields.io/badge/Gurubase-Ask%20GoFr%20Guru-006BFF?style=for-the-badge" /></a>
</div>

<h2 align="center">Listed in the <a href="https://landscape.cncf.io/?selected=go-fr">CNCF Landscape</a></h2>

## 🎯 **Goal**
GoFr is designed to **simplify microservice development**, with key focuses on **Kubernetes deployment** and **out-of-the-box observability**. While capable of building generic applications, **microservices** remain at its core.

---

## 💡 **Key Features**

1. **Simple API Syntax**
2. **REST Standards by Default**
3. **Configuration Management**
4. **[Observability](https://gofr.dev/docs/quick-start/observability)** (Logs, Traces, Metrics)
5. **Inbuilt [Auth Middleware](https://gofr.dev/docs/advanced-guide/http-authentication)** & Custom Middleware Support
6. **[gRPC Support](https://gofr.dev/docs/advanced-guide/grpc)**
7. **[HTTP Service](https://gofr.dev/docs/advanced-guide/http-communication)** with Circuit Breaker Support
8. **[Pub/Sub](https://gofr.dev/docs/advanced-guide/using-publisher-subscriber)**
9. **[Health Check](https://gofr.dev/docs/advanced-guide/monitoring-service-health)** for All Datasources
10. **[Database Migration](https://gofr.dev/docs/advanced-guide/handling-data-migrations)**
11. **[Cron Jobs](https://gofr.dev/docs/advanced-guide/using-cron)**
12. **Support for [Changing Log Level](https://gofr.dev/docs/advanced-guide/remote-log-level-change) Without Restarting**
13. **[Swagger Rendering](https://gofr.dev/docs/advanced-guide/swagger-documentation)**
14. **[Abstracted File Systems](https://gofr.dev/docs/advanced-guide/handling-file)**
15. **[Websockets](https://gofr.dev/docs/advanced-guide/handling-file)**

---

## 🚀 **Getting Started**

### **Prerequisites**
- GoFr requires **[Go](https://go.dev/)** version **[1.21](https://go.dev/doc/devel/release#go1.21.0)** or above.

### **Installation**
To get started with GoFr, add the following import to your code and use Go’s module support to automatically fetch dependencies:

```go
import "gofr.dev/pkg/gofr"
```

Alternatively, use the command:

```bash
go get -u gofr.dev/pkg/gofr
```

---

## 🏃 **Running GoFr**

Here's a simple example to get a GoFr application up and running:

```go
package main

import "gofr.dev/pkg/gofr"

func main() {
	app := gofr.New()

	app.GET("/greet", func(ctx *gofr.Context) (any, error) {
		return "Hello World!", nil
	})

	app.Run() // listens and serves on localhost:8000
}
```

To run this code:

```bash
$ go run main.go
```

Visit [`localhost:8000/greet`](http://localhost:8000/greet) to see the result.

---

## 📂 **More Examples**

Explore a variety of ready-to-run examples in the [GoFr examples directory](https://github.com/gofr-dev/gofr/tree/development/examples).

---

## 👩‍💻 **Documentation**

- **[GoDoc](https://pkg.go.dev/gofr.dev)**: Official API documentation.
- **[GoFr Documentation](https://gofr.dev/docs)**: Comprehensive guides and resources.

---

## 👍 **Contribute**

Help us make GoFr even better:

1. **Star** this repo on GitHub! 🌟
2. Write a review or tutorial on **[Medium](https://medium.com/)**, **[Dev.to](https://dev.to/)**, or your blog.
3. Review the **[CONTRIBUTING.md](CONTRIBUTING.md)** guide to learn how to contribute to the project.

---

## 🔒 **Secure Cloning**
To securely clone the GoFr repository, you can use HTTPS or SSH:

### Cloning with HTTPS
```bash
git clone https://github.com/gofr-dev/gofr.git
```
### Cloning with SSH
```bash
git clone git@github.com:gofr-dev/gofr.git
```

### 🎁 **Get a GoFr T-Shirt & Stickers!**

If your PR is merged, or if you contribute by writing articles or promoting GoFr, we invite you to fill out [this form](https://forms.gle/R1Yz7ZzY3U5WWTgy5) to claim your GoFr merchandise as a token of our appreciation! 

### Partners

<img src="https://resources.jetbrains.com/storage/products/company/brand/logos/jetbrains.png" alt="JetBrains logo" width="200">
