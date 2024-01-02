package file

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

type azure struct {
	fileName string
	fileMode Mode

	blockBlobURL azblob.BlockBlobURL
	blockSize    int64
	parallelism  uint16
}

func newAzureFile(c *AzureConfig, filename string, mode Mode) (*azure, error) {
	credential, err := azblob.NewSharedKeyCredential(c.AccountName, c.AccessKey)
	if err != nil {
		return nil, err
	}

	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	URL, err := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", c.AccountName, c.ContainerName))
	if err != nil {
		return nil, err
	}

	containerURL := azblob.NewContainerURL(*URL, p)
	blockBlobURL := containerURL.NewBlockBlobURL(filename)

	azFile := &azure{
		fileName:     filename,
		fileMode:     mode,
		blockBlobURL: blockBlobURL,
		blockSize:    blockSize,
		parallelism:  parallelism,
	}

	if c.BlockSize != "" {
		var bSize int

		if bSize, err = strconv.Atoi(c.BlockSize); err != nil {
			return nil, err
		}

		azFile.blockSize = int64(bSize)
	}

	if c.Parallelism != "" {
		var pl int

		if pl, err = strconv.Atoi(c.Parallelism); err != nil {
			return nil, err
		}

		azFile.parallelism = validateAndConvertToUint16(pl)
	}

	return azFile, nil
}

// need this implementation as this func is required by FTP
func (a *azure) move(string, string) error {
	return nil
}

func (a *azure) fetch(fd *os.File) error {
	err := azblob.DownloadBlobToFile(context.TODO(), a.blockBlobURL.BlobURL, 0, azblob.CountToEnd, fd,
		azblob.DownloadFromBlobOptions{
			BlockSize:   a.blockSize,
			Parallelism: a.parallelism,
		})

	return err
}

func (a *azure) push(fd *os.File) error {
	_, err := azblob.UploadFileToBlockBlob(context.TODO(), fd, a.blockBlobURL, azblob.UploadToBlockBlobOptions{
		BlockSize:   a.blockSize,
		Parallelism: a.parallelism,
	})

	return err
}

func (a *azure) list(string) ([]string, error) {
	return nil, ErrListingNotSupported
}

func randomString() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec //  Use of weak random number generator
	return strconv.Itoa(r.Int())
}

func validateAndConvertToUint16(num int) uint16 {
	if num >= 0 && num <= math.MaxUint16 {
		return uint16(num)
	}

	return uint16(0)
}
