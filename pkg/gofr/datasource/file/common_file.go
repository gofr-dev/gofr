package file

import (
	"io/fs"
	"time"
)

// CommonFile implements FileInfo for all providers, eliminating redundant metadata getters.
// Providers instantiate this struct when returning file metadata.
type CommonFile struct {
	name         string
	size         int64
	contentType  string
	lastModified time.Time
	isDir        bool
}

// Name returns the base name of the file.
func (f *CommonFile) Name() string {
	return f.name
}

// Size returns the file size in bytes. Returns 0 for directories.
func (f *CommonFile) Size() int64 {
	return f.size
}

// ModTime returns the last modification time.
func (f *CommonFile) ModTime() time.Time {
	return f.lastModified
}

// IsDir returns true if the object is a directory.
// Checks both explicit isDir flag and content type for compatibility.
func (f *CommonFile) IsDir() bool {
	return f.isDir || f.contentType == "application/x-directory"
}

// Mode returns the file mode. Directories return fs.ModeDir, files return 0.
func (f *CommonFile) Mode() fs.FileMode {
	if f.IsDir() {
		return fs.ModeDir
	}

	return 0
}

// Sys returns nil (no underlying system-specific data for cloud storage).
func (*CommonFile) Sys() any {
	return nil
}
