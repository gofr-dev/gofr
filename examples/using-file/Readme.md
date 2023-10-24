# USING FILE

This example demonstrates the read, write, list, and move functionality for different file stores (AWS, Azure, GCP, SFTP, FTP, LOCAL).

**Note:** move functionality is only supported for FTP and SFTP. 

To get started, set the `FILE_STORE` value to your desired file store type and configure other settings accordingly.

For reading a file , you should have a `test.txt` file in your file-store which will be containing the content to be read.

By default, it will read only 20 bytes, you need to set the number of bytes you want to read, in handler.

## Filestore Setup
1. To set up `FTP` locally, use the following docker command:
   ```bash
   docker run -d --name gofr-ftp -v $HOME/ftpData:/home/vsftpd -p 20:20 -p 21:21 -p 21100-21110:21100-21110 -e FTP_USER=myuser -e FTP_PASS=mypass -e PASV_ADDRESS=127.0.0.1 -e PASV_MIN_PORT=21100 -e PASV_MAX_PORT=21110 fauria/vsftpd
   ```
2. To set up `SFTP` locally, use the following docker command:
   ```bash
   docker run --name gofr-sftp -v $HOME/upload:/home/foo -p 2222:22 -d atmoz/sftp myuser:mypass:1001
   ```

## RUN
To run the app follow the below steps:

1. ` go run main.go [arg]`
2. `arg` can have either of the following value:
   - `read`: To read file (text.txt will be read from file store)
   - `write`: To write file (text.txt will be created in file store)
   - `list`: To list the contents of current directory of file store
   - `move -src=path/to/source/file -dest=path/to/destination/file`: To move file from `src` (source) to `dest` (destination)