# Handling File

GoFr simplifies the complexity of working with different file stores by offering a uniform API. This allows developers to interact with different storage systems using the same set of methods, without needing to understand the underlying implementation details of each file store.

## USAGE

By default, local file-store is initialised and user can access it from the context.

GoFr also supports FTP/SFTP file-store. Developers can also connect and use their AWS S3 bucket as a file-store. The file-store can be initialised as follows:

### FTP file-store
```go
package main

import (
	"gofr.dev/pkg/gofr"

	"gofr.dev/pkg/gofr/datasource/file/ftp"
)

func main() {
	app := gofr.New()

	app.AddFileStore(ftp.New(&ftp.Config{
		Host:      "127.0.0.1",
		User:      "user",
		Password:  "password",
		Port:      21,
		RemoteDir: "/ftp/user",
	}))

	app.Run()
}
```

### SFTP file-store
```go
package main

import (
	"gofr.dev/pkg/gofr"

	"gofr.dev/pkg/gofr/datasource/file/sftp"
)

func main() {
	app := gofr.New()

	app.AddFileStore(sftp.New(&sftp.Config{
		Host:     "127.0.0.1",
		User:     "user",
		Password: "password",
		Port:     22,
	}))

	app.Run()
}
```

### AWS S3 Bucket as File-Store

To run S3 File-Store locally we can use localstack,
``docker run --rm -it -p 4566:4566 -p 4510-4559:4510-4559 localstack/localstack``


```go
package main

import (
	"gofr.dev/pkg/gofr"

	"gofr.dev/pkg/gofr/datasource/file/s3"
)

func main() {
	app := gofr.New()

	// Note that currently we do not handle connections through session token.
	// BaseEndpoint is not necessary while connecting to AWS as it automatically resolves it on the basis of region.
	// However, in case we are using any other AWS compatible service, such like running or testing locally, then this needs to be set.
	// Note that locally, AccessKeyID & SecretAccessKey is not checked if we use localstack.
	app.AddFileStore(s3.New(&s3.Config{
		EndPoint:        "http://localhost:4566",
		BucketName:      "gofr-bucket-2",
		Region:          "us-east-1",
		AccessKeyID:     app.Config.Get("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: app.Config.Get("AWS_SECRET_ACCESS_KEY"),
	}))

	app.Run()
}
```
> Note: The current implementation supports handling only one bucket at a time, 
> as shown in the example with `gofr-bucket-2`. Bucket switching mid-operation is not supported. 

### Creating Directory

To create a single directory
```go
err := ctx.File.Mkdir("my_dir",os.ModePerm)
```

To create subdirectories as well
```go
err := ctx.File.MkdirAll("my_dir/sub_dir", os.ModePerm)
```

### Get current Directory

```go
currentDir, err := ctx.File.Getwd()
```

### Change current Directory

To switch to parent directory
```go
currentDir, err := ctx.File.Chdir("..")
```

To switch to another directory in same parent directory
```go
currentDir, err := ctx.File.Chdir("../my_dir2")
```

To switch to a subfolder of the current directory
```go
currentDir, err := ctx.File.Chdir("sub_dir")
```
> Note: This method attempts to change the directory, but S3's flat structure and fixed bucket
> make this operation inapplicable.

### Read a Directory

The ReadDir function reads the specified directory and returns a sorted list of its entries as FileInfo objects. Each FileInfo object provides access to its associated methods, eliminating the need for additional stat calls. 

If an error occurs during the read operation, ReadDir returns the successfully read entries up to the point of the error along with the error itself. Passing "." as the directory argument returns the entries for the current directory.
```go
entries, err := ctx.File.ReadDir("../testdir")

for _, entry := range entries {
    entryType := "File"

    if entry.IsDir() {
        entryType = "Dir"
    } 

    fmt.Printf("%v: %v Size: %v Last Modified Time : %v\n" entryType, entry.Name(), entry.Size(), entry.ModTime())
}
```
> Note: In S3, directories are represented as prefixes of file keys. This method retrieves file
> entries only from the immediate level within the specified directory.

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
	// In case of JSON user should pass structs with JSON tags as defined in encoding/json.
	err = reader.Scan(&b)

	fmt.Println(b)
}
```


### Opening and Reading Content from a File
To open a file with default settings, use the `Open` command, which provides read and seek permissions only. For write permissions, use `OpenFile` with the appropriate file modes.
> Note: In FTP, file permissions are not differentiated; both `Open` and `OpenFile` allow all file operations regardless of specified permissions.
```go
csvFile, _ := ctx.File.Open("my_file.csv")

b := make([]byte, 200)

// Read reads up to len(b) bytes into b.
_, _ = file.Read(b)

csvFile.Close()

csvFile, err = ctx.File.OpenFile("my_file.csv", os.O_RDWR, os.ModePerm)

// WriteAt writes the buffer content at the specified offset.
_, err = csvFile.WriteAt([]byte("test content"), 4)
if err != nil {
     return nil, err
}
```

### Getting Information of the file/directory

Stat retrieves details of a file or directory, including its name, size, last modified time, and type (such as whether it is a file or folder)
```go
file, _ := ctx.File.Stat("my_file.text")
entryType := "File"

if entry.IsDir() {
     entryType = "Dir"
}

fmt.Printf("%v: %v Size: %v Last Modified Time : %v\n" entryType, entry.Name(), entry.Size(), entry.ModTime())
```
>Note: In S3:
> - Names without a file extension are treated as directories by default.
> - Names starting with "0" are interpreted as binary files, with the "0" prefix removed.
>   
> For directories, the method calculates the total size of all contained objects and returns the most recent modification time. For files, it directly returns the file's size and last modified time.

### Rename/Move a File

To rename or move a file, provide source and destination fields.
In case of renaming a file provide current name as source, new_name in destination.
To move file from one location to another provide current location as source and new location as destination.

```go
err := ctx.File.Rename("old_name.text", "new_name.text")
```

### Deleting Files

`Remove` deletes a single file
> Note: Currently, the S3 package supports the deletion of unversioned files from general-purpose buckets only. Directory buckets and versioned files are not supported for deletion by this method.
```go
err := ctx.File.Remove("my_dir")
```

The `RemoveAll` command deletes all subdirectories as well. If you delete the current working directory, such as "../currentDir", the working directory will be reset to its parent directory.
> Note: In S3, RemoveAll only supports deleting directories and will return an error if a file path (as indicated by a file extension) is provided.
```go
err := ctx.File.RemoveAll("my_dir/my_text")
```

> GoFr supports relative paths, allowing locations to be referenced relative to the current working directory. However, since S3 uses
> a flat file structure, all methods require a full path relative to the S3 bucket.

> Errors have been skipped in the example to focus on the core logic, it is recommended to handle all the errors.
