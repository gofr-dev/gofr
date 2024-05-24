package ftp

import (
	"bytes"
	goftp "github.com/jlaffaye/ftp"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics"
	"io"
)

type ftp struct {
	logger  datasource.Logger
	metrics metrics.Manager
	cfg     Config

	conn *goftp.ServerConn
}

type Config struct {
	Host     string
	Port     string
	Username string
	Password string
}

func New(cfg Config) datasource.FileStoreProvider {
	var f ftp

	f.cfg = cfg

	return &f
}

func (f *ftp) UseLogger(logger interface{}) {
	f.logger = logger.(logging.Logger)
}

func (f *ftp) UseMetrics(m interface{}) {
	f.metrics = m.(metrics.Manager)
}

func (f *ftp) Connect() {
	conn, err := goftp.Dial(f.cfg.Host + ":" + f.cfg.Port)
	if err != nil {
		f.logger.Errorf("Failed to connect to FTP: %v", err)
		return
	}

	err = conn.Login(f.cfg.Username, f.cfg.Password)
	if err != nil {
		f.logger.Errorf("Failed to login to FTP: %v", err)
		return
	}

	f.conn = conn
}

// CreateDir creates a new directory with the specified named name,
// along with any necessary parents with fs.ModeDir FileMode.
// If directory already exist it will do nothing and return nil.
// name contains the file name along with the path.
func (f *ftp) CreateDir(name string, option ...interface{}) error {
	return f.conn.MakeDir(name)
}

// Create creates the file named path along with any necessary parents,
// and writes the given data to it.
// If file exists, error is returned.
// If file does not exist, it is created with mode 0666
// Error return are of type *fs.PathError.
// name contains the file name along with the path.
func (f *ftp) Create(name string, data []byte, option ...interface{}) error {
	// Open the file for writing in binary mode
	err := f.conn.Stor(name, bytes.NewReader(data))
	if err != nil {
		return err
	}

	return nil
}

// Read reads the content of file and writes it in data.
// If there is an error, it will be of type *fs.PathError.
// name contains the file name along with the path.
func (f *ftp) Read(name string, option ...interface{}) ([]byte, error) {
	resp, err := f.conn.Retr(name)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Move moves the file from src to dest, along with any necessary parents for dest location.
// If there is an error, it will be of type *fs.PathError.
// src and dest contains the filename along with path
func (f *ftp) Move(src string, dest string, option ...interface{}) error {
	return f.conn.Rename(src, dest)
}

// Update rewrites file named path with data, if file doesn't exist, error is returned.
// name contains the file name along with the path.
func (f *ftp) Update(name string, data []byte, option ...interface{}) error {
	err := f.Delete(name)
	if err != nil {
		return err
	}

	return f.Create(name, data, option)

}

// Delete deletes the file at given path, if no file/directory exist nil is returned.
// name contains the file name along with the path.
func (f *ftp) Delete(name string, option ...interface{}) error {
	return f.conn.Delete(name)
}

//// Stat returns stat for the file.
//func (f *ftp) Stat(name string, options ...interface{}) (fs.FileInfo, error) {
//	var isFile bool
//	var entries []*goftp.Entry
//	var fileList []string
//	var err error
//
//	if !strings.HasSuffix(name, ".*") {
//		isFile = true
//	}
//
//	fileList = strings.Split(name, "/")
//
//	if !isFile {
//		entries, err = f.conn.List(strings.Join(fileList[:len(fileList)-1], "/"))
//		fmt.Println(entries)
//		fmt.Println(err)
//
//	}
//
//	size, err := f.conn.FileSize(name)
//
//	info := ftpFileInfo{
//		FName:    entries[0].Name,
//		FSize:    size,
//		FModTime: entries[0].Time,
//	}
//
//	if entries[0].Type == 1 {
//		info.FIsDir = true
//	}
//
//	return &info, nil
//}
//
//type ftpFileInfo struct {
//	FName    string
//	FSize    int64
//	FMode    fs.FileMode
//	FModTime time.Time
//	FIsDir   bool
//}
//
//func (f *ftpFileInfo) Name() string {
//	return f.FName
//}
//
//func (f *ftpFileInfo) Size() int64 {
//	return f.FSize
//}
//
//func (f *ftpFileInfo) Mode() fs.FileMode {
//	return f.FMode
//}
//
//func (f *ftpFileInfo) ModTime() time.Time {
//	return f.FModTime
//}
//
//func (f *ftpFileInfo) IsDir() bool {
//	return f.FIsDir
//}
//
//func (f *ftpFileInfo) Sys() any {
//	return nil
//}
