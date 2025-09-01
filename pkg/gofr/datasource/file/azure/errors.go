package azure

import "errors"

// Static errors for Azure file operations.
var (
	ErrCreateDirectoryNotImplemented                = errors.New("CreateDirectory not implemented")
	ErrDeleteDirectoryNotImplemented                = errors.New("DeleteDirectory not implemented")
	ErrListFilesAndDirectoriesSegmentNotImplemented = errors.New("ListFilesAndDirectoriesSegment not implemented")
	ErrCreateFileNotImplemented                     = errors.New("CreateFile not implemented")
	ErrDeleteFileNotImplemented                     = errors.New("DeleteFile not implemented")
	ErrDownloadFileNotImplemented                   = errors.New("DownloadFile not implemented")
	ErrUploadRangeNotImplemented                    = errors.New("UploadRange not implemented")
	ErrGetPropertiesNotImplemented                  = errors.New("GetProperties not implemented")
	ErrRemoveNotImplemented                         = errors.New("Remove not implemented")
	ErrRenameNotImplemented                         = errors.New("rename operation not implemented for Azure File Storage")
	ErrMkdirNotImplemented                          = errors.New("Mkdir not implemented")
	ErrReadDirNotImplemented                        = errors.New("ReadDir not implemented")
	ErrChDirNotImplemented                          = errors.New("ChDir not implemented for Azure File Storage")
	ErrReadNotImplemented                           = errors.New("Read not implemented")
	ErrWriteNotImplemented                          = errors.New("Write not implemented")
	ErrInvalidWhence                                = errors.New("invalid whence")
	ErrShareClientNotInitialized                    = errors.New("share client not initialized")
	ErrDownloadFileRequiresPath                     = errors.New("download file requires file path context")
	ErrUploadRangeRequiresPath                      = errors.New("upload range requires file path context")
	ErrGetPropertiesRequiresPath                    = errors.New("get properties requires file path context")
)
