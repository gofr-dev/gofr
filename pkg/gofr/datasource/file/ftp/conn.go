package ftp

import "github.com/jlaffaye/ftp"

type Conn struct {
	*ftp.ServerConn
}

func (c *Conn) Retr(path string) (ftpResponse, error) {
	return c.ServerConn.Retr(path)
}

func (c *Conn) RetrFrom(path string, offset uint64) (ftpResponse, error) {
	return c.ServerConn.RetrFrom(path, offset)
}
