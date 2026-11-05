package s3

import "errors"

// ErrNotConnected is returned when a file operation is attempted before the
// S3 client has been successfully initialized by Connect.
var ErrNotConnected = errors.New("s3 file store is not connected")
