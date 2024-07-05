# Using File Bind Example

This GoFr example demonstrates the use of context Bind where incoming request has multipart-form data and then binds
it to the fields of the struct. GoFr currently supports zip file type and also binds the more generic [`multipart.FileHeader`](https://pkg.go.dev/mime/multipart#FileHeader)

### Usage
```go
type Data struct {
    Compressed file.Zip `file:"upload"`

    FileHeader *multipart.FileHeader `file:"a"`
}

func Handler (c *gofr.Context) (interface{}, error) {
    var d Data
    
    // bind the multipart data into the variable d
    err := c.Bind(&d)
    if err != nil {
        return nil, err
    }
}
```

### To run the example use the command below:
```console
go run main.go
```
