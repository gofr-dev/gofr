# Filestore
GoFr supports the following:
 - FTP
 - SFTP
 - AWS 
 - GCP
 - LOCAL

To incorporate these, the respective configs must be set. Please refer to  [configs](/docs/new/configuration/introduction).

## Initialising Filestore
```go
// initialises file store, file store can be switched by changing configs
file, err := file.NewWithConfig(app.Config, "test.txt", "rw")
if err != nil {
	app.Logger.Error("Unable to initialize", err)
	return
}
```

## Usage
**Reading a File**
```go
err := file.Open()
if err != nil {
	return nil, err
}

defer file.Close()

// byte stream where data has to saved after reading
b := make([]byte, size)

_, err = file.Read(b)
if err != nil {
	return nil, err
}
```

**Writing to a File**
```go
err := file.Open()
if err != nil {
	return err
}

b := []byte("Welcome to GoFr!")

_, err = file.Write(b)
if err != nil {
	return err
}

err = file.Close()
if err != nil {
	return nil, err
}
```
**List Files in Directory**
```go
files, err := file.List(".")
```
**Move Files From Source to Destination**
```go
err := file.Move(src, dest)
if err != nil {
	return nil, err
}
```