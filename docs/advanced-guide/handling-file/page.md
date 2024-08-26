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

### Rename/Move a File

To rename or move a file, provide source and destination fields.
In case of renaming a file provide current name as source, new_name in destination.
To move file from one location to another provide current location as source and new location as destination.

```go
err := ctx.File.Rename("old_name.text", "new_name.text")
```

### Deleting Files

Remove deletes a single file
```go
err := ctx.File.Remove("my_dir")
```

The `RemoveAll` command deletes all subdirectories as well. If you delete the current working directory, such as "../currentDir", the working directory will be reset to its parent directory.
```go
err := ctx.File.RemoveAll("my_dir/my_text")
```

> GoFr supports relative paths i.e. a location relative to the current working directory. The resolution of this path depends on the current directory from which the path is being referenced. 

> Errors have been skipped in the example to focus on the core logic, it is recommended to handle all the errors.
