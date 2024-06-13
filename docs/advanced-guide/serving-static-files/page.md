# Serving Static Files using GoFr

Often, we are required to serve static content such as a default profile image, a favicon, or a background image for our 
web application. We want to have a mechanism to serve that static content without the hassle of implementing it from scratch.

GoFr provides a default mechanism where if a `static` folder is available in the directory of the application,
it automatically provides an endpoint with `/static/<filename>`, here filename refers to the file we want to get static content to be served. 

Example project structure:

```dotenv
project_folder
|
|---configs
|       .env
|---static
|       img1.jpeg
|       img2.png
|       img3.jpeg
|   main.go
|   main_test.go
```

main.go code:

```go
package main

import "gofr.dev/pkg/gofr"

func main(){
    app := gofr.New()
    app.Run()
}
```

Additionally, if we want to serve more static endpoints, we have a dedicated function called `AddStaticFiles()`
which takes 2 parameters `endpoint` and the `filepath` of the static folder which we want to serve.

Example project structure:

```dotenv
project_folder
|
|---configs
|       .env
|---static
|       img1.jpeg
|       img2.png
|       img3.jpeg
|---public
|       |---css
|       |       main.css
|       |---js
|       |       main.js
|       |   index.html
|   main.go
|   main_test.go
```

main.go file:

```go
package main

import "gofr.dev/pkg/gofr"

func main(){
    app := gofr.New()
    app.AddStaticFiles("public", "./public")
    app.Run()
}
```

In the above example, both endpoints `/public` and `/static` are available for the app to render the static content.
