package ftp

import (
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/metrics"
	"io/fs"

	goftp "github.com/jlaffaye/ftp"
)

type ftp struct {
	logger  datasource.Logger
	metrics metrics.Manager
	cfg     Config

	conn *goftp.ServerConn
}

type Config struct {
	Host string
}

func New(cfg Config) datasource.FileStoreProvider {
	var f ftp

	conn, err := goftp.Dial(cfg.Host)
	if err != nil {
		return nil
	}

	return f
}

func (f ftp) UseLogger(logger interface{}) {
	//TODO implement me
	panic("implement me")
}

func (f ftp) UseMetrics(metrics interface{}) {
	//TODO implement me
	panic("implement me")
}

func (f ftp) Connect() {
	//TODO implement me
	panic("implement me")
}

func (f ftp) CreateDir(name string, options ...interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (f ftp) Create(name string, data []byte, options ...interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (f ftp) Read(name string, options ...interface{}) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (f ftp) Move(src string, dest string, options ...interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (f ftp) Update(name string, data []byte, options ...interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (f ftp) Delete(name string, options ...interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (f ftp) Stat(name string, options ...interface{}) (fs.FileInfo, error) {
	//TODO implement me
	panic("implement me")
}
