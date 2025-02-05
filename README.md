# <h1 align="center" style="font-size: 100px; font-weight: 500;">🚀 <i>Go</i>Fr</h1>

<p align="center">
  <img align="center" width="300" alt="logo" src="https://github.com/gofr-dev/gofr/assets/44036979/916fe7b1-42fb-4af1-9e0b-4a7a064c243c">
</p>

<h2 align="center">The Ultimate Opinionated Microservice Development Framework 🔥</h2>

<div align="center">
<a href="https://pkg.go.dev/gofr.dev"><img src="https://img.shields.io/badge/GoDoc-Read%20Docs-blue?style=for-the-badge" alt="godoc"></a>
<a href="https://gofr.dev/docs"><img src="https://img.shields.io/badge/GoFr-Docs-orange?style=for-the-badge" alt="gofr-docs"></a>
<a href="https://codeclimate.com/github/gofr-dev/gofr/maintainability"><img src="https://img.shields.io/codeclimate/maintainability/gofr-dev/gofr?style=for-the-badge" alt="maintainability"></a>
<a href="https://goreportcard.com/report/gofr.dev"><img src="https://goreportcard.com/badge/gofr.dev?style=for-the-badge" alt="Go Report Card"></a>
<a href="https://discord.gg/wsaSkQTdgq"><img src="https://img.shields.io/badge/discord-join-us?style=for-the-badge&logo=discord&color=7289DA" alt="discord" /></a>
</div>

---

## 🎯 **Why GoFr?**
GoFr isn't just another microservice framework. It's designed for **effortless scalability**, **observability**, and **Kubernetes-friendly deployments**—so you can focus on building great services instead of wrestling with infrastructure. 💪

---

## 🔥 **Key Features That Make GoFr Awesome**

✅ **Lightning-fast API Development** with a simple, clean syntax ⚡  
✅ **REST-first** architecture for seamless integrations 🌎  
✅ **Built-in Observability** (Logging, Metrics, Tracing) 🔍  
✅ **Inbuilt Auth & Middleware Support** 🔐  
✅ **First-class Support for gRPC & Websockets** 🚀  
✅ **Database Migration & Health Checks** for peace of mind 🏥  
✅ **Auto-generating Swagger Docs** for easy API documentation 📄  
✅ **Dynamic Log Level Changes** without restarting! 🛠️  
✅ **Supports Pub/Sub, Cron Jobs, & Abstracted File Systems** 🎯

---

## 🚀 **Getting Started in 60 Seconds!**

### **Prerequisites**
- Install **[Go](https://go.dev/)** (v1.21+)

### **Installation**
```bash
go get -u gofr.dev/pkg/gofr
```

### **First GoFr App!** 🏗️
```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    app := gofr.New()

    app.GET("/greet", func(ctx *gofr.Context) (any, error) {
        return "Hello, GoFr World! 🌍", nil
    })

    app.Run() // Serves on localhost:8000
}
```

### **Run Your App**
```bash
go run main.go
```
Visit [`localhost:8000/greet`](http://localhost:8000/greet) and say hello to GoFr! 🎉

---

## 📚 **More Learning Resources**
- 📖 **[GoDoc](https://pkg.go.dev/gofr.dev)** - API Reference
- 📘 **[GoFr Documentation](https://gofr.dev/docs)** - Step-by-step guides
- 🔥 **[Example Projects](https://github.com/gofr-dev/gofr/tree/development/examples)** - Learn by doing!

---

## 🌟 **Join the GoFr Community!**

🚀 **Contribute & Win!** We love our contributors! Help improve GoFr and **win exclusive swag** (T-shirts, stickers, and more!). 🎁

### **Ways to Contribute**:
✔️ **Star this repo** ⭐ to support the project  
✔️ Write a blog/tutorial on **GoFr** and share it  
✔️ Submit PRs and improvements (Check **[CONTRIBUTING.md](CONTRIBUTING.md)**)  
✔️ Engage with us on **[Discord](https://discord.gg/wsaSkQTdgq)**

---

## 🔒 **Secure Cloning & Repo Setup**

### Clone via HTTPS
```bash
git clone https://github.com/gofr-dev/gofr.git
```

### Clone via SSH
```bash
git clone git@github.com:gofr-dev/gofr.git
```

---

## 🎁 **Claim Your GoFr Swag!**
Love GoFr? Show it! If your **PR is merged**, or if you **write articles/tutorials** about GoFr, you can claim **exclusive GoFr T-shirts & Stickers**! Fill out [this form](https://forms.gle/R1Yz7ZzY3U5WWTgy5) to grab yours! 🚀

### Special Thanks to Our Partners ❤️

<p align="center">
  <img src="https://resources.jetbrains.com/storage/products/company/brand/logos/jetbrains.png" alt="JetBrains logo" width="200">
</p>

🔗 **CNCF Landscape Listing**: GoFr is officially listed in the **[CNCF Landscape](https://landscape.cncf.io/?selected=go-fr)** 🌍

---

## 🚀 **Built for the Future, Ready Today!**

🔥 Whether you're building a startup, scaling microservices, or creating the next big SaaS product, **GoFr has your back!** Try it today and join our fast-growing community. 🏆
