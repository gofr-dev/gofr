package ftp

//
//import (
//	"bytes"
//	"fmt"
//	"gofr.dev/pkg/gofr/config"
//	"io/fs"
//
//	"github.com/jlaffaye/ftp"
//
//	"gofr.dev/pkg/gofr/datasource/file"
//)
//
//type Config struct {
//	Host     string
//	Port     string
//	UserName string
//	Password string
//
//	client *ftp.ServerConn
//}
//
//func New(config config.Config) File {
//	conn, err := ftp.Dial(fmt.Sprintf("%v:%v", config.Host, config.Port))
//	if err != nil {
//		return nil
//	}
//
//	config.client = conn
//
//	return config
//}
//
//func (c Config) CreateDir(path string) error {
//	return c.client.MakeDir(path)
//}
//
//func (c Config) Create(path string, data []byte) error {
//	return nil
//}
//
//func (c Config) Update(path string, data []byte) error {
//	err := c.client.Rename(path, "temp_"+path)
//	if err != nil {
//		return err
//	}
//
//	err = c.client.Stor(path, bytes.NewReader(data))
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func (c Config) Delete(path string) error {
//	return nil
//}
//
//func (c Config) Read(name string) ([]byte, error) {
//	return nil, nil
//}
//
//func (c Config) Move(src string, dest string) error {
//	return nil
//}
//
//func (c Config) Stat(name string) (fs.FileInfo, error) {
//	return nil, nil
//}
