# Add FileStore Example

This GoFr example demonstrates a CMD application that can be used to interact with a remote file server using FTP or SFTP protocol

### Setting up an FTP server in local machine
- https://security.appspot.com/vsftpd.html
- https://pypi.org/project/pyftpdlib/

Choose a library listed above and follow their respective documentation to configure an FTP server in your local machine and replace the configs/env file with correct HOST,USER_NAME,PASSWORD,PORT and REMOTE_DIR_PATH details.

### To run the example use the commands below:
To print the current working directory of the configured remote file server
```console
go run main.go pwd
```
To get the list of all directories or files in the given path of the configured remote file server

```
 go run main.go ls -path=/
```
To grep the list of all files and directories in the given path that is matching with the keyword provided

```
go run main.go grep -keyword=fi -path=/
```

To create a file in the current working directory with the provided filename 
```
 go run main.go createfile -filename=file.txt
```

To remove the file with the provided filename from the current working directory
```
 go run main.go rm -filename=file.txt
```