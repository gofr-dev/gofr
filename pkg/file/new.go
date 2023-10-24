package file

import (
	"os"
	"strconv"
	"time"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

const (
	Local = "LOCAL"
	Azure = "AZURE"
	AWS   = "AWS"
	GCP   = "GCP"
	SFTP  = "SFTP"
	FTP   = "FTP"
)

type Mode string

const (
	READ      = Mode("r")
	WRITE     = Mode("w")
	READWRITE = Mode("rw")
	APPEND    = Mode("a")
)

// Config is redefined in this package similar to what defined in gofr package.
type Config struct {
	FileStore string
	Azure     AzureConfig
	AWS       AWSConfig
	GCP       GCPConfig
	SFTP      SFTPConfig
	FTP       FTPConfig
}

// AzureConfig is used to store configurations related to Azure cloud storage.
type AzureConfig struct {
	AccountName   string
	AccessKey     string
	ContainerName string
	BlockSize     string
	Parallelism   string
}

type AWSConfig struct {
	AccessKey string
	SecretKey string
	Token     string
	Bucket    string
	Region    string
}

// GCPConfig is used to store configurations related to GCP cloud storage.
type GCPConfig struct {
	GCPKey     string
	BucketName string
}

// SFTPConfig is used to store configuration related to SFTP.
type SFTPConfig struct {
	Host     string
	User     string
	Password string
	Port     int
}

// FTPConfig is used to store configuration related to FTP.
type FTPConfig struct {
	Host          string
	User          string
	Password      string
	Port          int
	RetryDuration time.Duration
}

// NewWithConfig takes the gofr config and creates Config struct specific to this file package and then calls New()
func NewWithConfig(config gofr.Config, filename string, mode Mode) (Storage, error) {
	var fileConfig Config
	fileConfig.FileStore = config.Get("FILE_STORE")

	// Reading Azure Configs.
	fileConfig.Azure = setAzureConfig(config)

	// Reading AWS config
	fileConfig.AWS = setAWSConfig(config)

	// Reading GCP Configs.
	fileConfig.GCP = setGCPConfig(config)

	// Reading SFTP Configs
	fileConfig.SFTP = setSFTPConfig(config)

	// Reading FTP Configs
	fileConfig.FTP = setFTPConfig(config)

	return New(&fileConfig, filename, mode)
}

// New takes  file specific config struct and calls respective constructor functions for opening files
func New(config *Config, filename string, mode Mode) (Storage, error) {
	l := fileAbstractor{}

	switch config.FileStore {
	case Local:
		return newLocalFile(filename, mode), nil
	case Azure:
		azFile, err := newAzureFile(&config.Azure, filename, mode)
		l.fileName, l.fileMode, l.remoteFileAbstracter = filename, fetchLocalFileMode(mode), azFile

		return &l, err

	case AWS:
		awsFile := newAWSS3File(&config.AWS, filename, mode)
		l.fileName, l.fileMode, l.remoteFileAbstracter = filename, fetchLocalFileMode(mode), awsFile

		return &l, nil

	case GCP:
		gcpFile, err := newGCPFile(&config.GCP, filename, mode)
		l.fileName, l.fileMode, l.remoteFileAbstracter = filename, fetchLocalFileMode(mode), gcpFile

		return &l, err

	case SFTP:
		sftpFile, err := newSFTPFile(&config.SFTP, filename, mode)
		l.fileName, l.fileMode, l.remoteFileAbstracter = filename, fetchLocalFileMode(mode), sftpFile

		return &l, err

	case FTP:
		ftpFile, err := newFTPFile(&config.FTP, filename, mode)
		l.fileName, l.fileMode, l.remoteFileAbstracter = filename, fetchLocalFileMode(mode), ftpFile

		return &l, err

	default:
		return nil, errors.InvalidFileStorage
	}
}

func fetchLocalFileMode(mode Mode) int {
	var m int

	switch mode {
	case READ:
		m = os.O_RDONLY
	case WRITE:
		m = os.O_CREATE | os.O_WRONLY
	case READWRITE:
		m = os.O_CREATE | os.O_RDWR
	case APPEND:
		m = os.O_CREATE | os.O_APPEND | os.O_WRONLY
	}

	return m
}

func setAzureConfig(config gofr.Config) AzureConfig {
	return AzureConfig{
		AccountName:   config.Get("AZURE_STORAGE_ACCOUNT"),
		AccessKey:     config.Get("AZURE_STORAGE_ACCESS_KEY"),
		ContainerName: config.Get("AZURE_STORAGE_CONTAINER"),
		BlockSize:     config.Get("AZURE_STORAGE_BLOCK_SIZE"),
		Parallelism:   config.Get("AZURE_STORAGE_PARALLELISM"),
	}
}

func setAWSConfig(config gofr.Config) AWSConfig {
	return AWSConfig{
		AccessKey: config.Get("AWS_STORAGE_ACCESS_KEY"),
		SecretKey: config.Get("AWS_STORAGE_SECRET_KEY"),
		Token:     config.Get("AWS_STORAGE_TOKEN"),
		Bucket:    config.Get("AWS_STORAGE_BUCKET"),
		Region:    config.Get("AWS_STORAGE_REGION"),
	}
}

func setGCPConfig(config gofr.Config) GCPConfig {
	return GCPConfig{
		GCPKey:     config.Get("GCP_STORAGE_CREDENTIALS"),
		BucketName: config.Get("GCP_STORAGE_BUCKET_NAME"),
	}
}

func setSFTPConfig(config gofr.Config) SFTPConfig {
	port, _ := strconv.Atoi(config.Get("SFTP_PORT"))

	return SFTPConfig{
		Host:     config.Get("SFTP_HOST"),
		User:     config.Get("SFTP_USER"),
		Password: config.Get("SFTP_PASSWORD"),
		Port:     port,
	}
}

// setFTPConfig to set the FTP configs from env
func setFTPConfig(config gofr.Config) FTPConfig {
	port, err := strconv.Atoi(config.Get("FTP_PORT"))
	if err != nil {
		port = 21
	}

	return FTPConfig{
		Host:          config.Get("FTP_HOST"),
		User:          config.Get("FTP_USER"),
		Password:      config.Get("FTP_PASSWORD"),
		Port:          port,
		RetryDuration: getRetryDuration(config.Get("FTP_RETRY_DURATION")),
	}
}
