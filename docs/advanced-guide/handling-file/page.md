# Handling File

GoFr simplifies the complexity of working with different file stores by offering a uniform API. This allows developers to interact with different storage systems using the same set of methods, without needing to understand the underlying implementation details of each file store.

## USAGE

By default, local file-store is initialised and user can access it from the context.

Gofr also supports FTP file-store. The file-store can be initialised with FTP as follows:

### FTP file-store
```go
package main

import (
    "gofr.dev/pkg/gofr"

    "gofr.dev/pkg/gofr/datasource/file/ftp"
)

func main() {
    app := gofr.New()

	app.AddFTP(ftp.New(&ftp.Config{
		Host:      "127.0.0.1",
		User:      "user",
		Password:  "password",
		Port:      21,
		RemoteDir: "/ftp/user",
	}))
    
    app.Run()
}
```


### Creating Directory

To create a single directory
```go
err := ctx.File.Mkdir("my_dir",os.ModePerm)
```

To create subdirectories as well
```go
err := ctx.File.MkdirAll("my_dir/sub_dir", os.ModePerm)
```

### Creating and Save a File with Content

```go
file, _ := ctx.File.Create("my_file.text")

_, _ = file.Write([]byte("Hello World!"))

// Closes and saves the file.
	file.Close()
```

### Reading file as CSV/JSON/TEXT
GoFr support reading CSV/JSON/TEXT files line by line.

```go
reader, err := file.ReadAll()

for reader.Next() {
	var b string
		
	// For reading CSV/TEXT files user need to pass pointer to string to SCAN.
	// Incase of JSON user should pass structs with JSON tags as defined in encoding/json.
	err = reader.Scan(&b)

	fmt.Println(b)
}
```


### Opening and Reading Content from a File

To open file with default options
```go
file, _ := ctx.File.Open("my_file.text")
defer file.Close()

b := make([]byte, 200)

// Read reads up to len(b) bytes into b
_, _ = file.Read(b)
```

### Rename/Move a File

To rename or move a file, provide source and destination fields.
In case of renaming a file provide current name as source, new_name in destination.
To move file from one location to another provide current location as source and new location as destination.

```go
err := ctx.File.Rename("old_name.text", "new_name.text")
```

### Deleting Files
To delete a single file
```go
err := ctx.File.Remove("my_dir")
```

To delete all sub directories as well
```go
err := ctx.File.RemoveAll("my_dir/my_text")
```


> Errors have been skipped in the example to focus on the core logic, it is recommended to handle all the errors.
