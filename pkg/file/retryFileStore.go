package file

import (
	"strconv"
	"time"
)

const defaultRetryDuration = 5

// retryFTP to retry FTP connection
func retryFTP(c *FTPConfig, f *ftp) {
	for {
		time.Sleep(c.RetryDuration * time.Second)

		if f.conn != nil {
			conn, ok := f.conn.(*ftpConn)
			if !ok {
				return
			}

			if conn.conn.NoOp() != nil {
				_ = connectFTP(c, f)
			}
		} else {
			_ = connectFTP(c, f)
		}
	}
}

// getRetryDuration utility function to get retry duration as string and return as time.duration
// If there is any error in parsing retry duration then default retry duration will be returned
func getRetryDuration(duration string) time.Duration {
	retryDuration, err := strconv.Atoi(duration)
	if err != nil {
		retryDuration = defaultRetryDuration
	}

	return time.Duration(retryDuration)
}
