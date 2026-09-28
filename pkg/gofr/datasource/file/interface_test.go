package file

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// cloudFS mirrors how cloud providers wrap CommonFileSystem and expose the provider-level Connect.
type cloudFS struct {
	*CommonFileSystem
}

func (cloudFS) Connect() {}

func TestAsCloud(t *testing.T) {
	tests := []struct {
		name  string
		fs    func(ctrl *gomock.Controller) FileSystem
		expOK bool
	}{
		{
			name:  "provider built on common file system supports cloud operations",
			fs:    func(*gomock.Controller) FileSystem { return cloudFS{CommonFileSystem: &CommonFileSystem{}} },
			expOK: true,
		},
		{
			name:  "plain file system is not a cloud file system",
			fs:    func(ctrl *gomock.Controller) FileSystem { return NewMockFileSystem(ctrl) },
			expOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := tt.fs(gomock.NewController(t))

			cfs, ok := AsCloud(fs)

			assert.Equal(t, tt.expOK, ok)
			assert.Equal(t, tt.expOK, cfs != nil)
		})
	}
}
