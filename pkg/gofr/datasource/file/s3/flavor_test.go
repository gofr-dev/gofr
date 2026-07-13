package s3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func boolPtr(b bool) *bool { return &b }

func Test_resolveProfile(t *testing.T) {
	tests := []struct {
		name                string
		config              *Config
		wantPathStyle       bool
		wantSigningRegion   string
		wantDisableChecksum bool
		wantEffectiveRegion string
	}{
		{
			name:                "AWS default (empty flavor) keeps path-style",
			config:              &Config{Region: "us-east-1"},
			wantPathStyle:       true,
			wantEffectiveRegion: "us-east-1",
		},
		{
			name:                "R2 signs with auto region and disables checksum",
			config:              &Config{Flavor: FlavorR2, Region: "us-east-1"},
			wantPathStyle:       true,
			wantSigningRegion:   "auto",
			wantDisableChecksum: true,
			wantEffectiveRegion: "auto",
		},
		{
			name:                "MinIO uses path-style and disables checksum",
			config:              &Config{Flavor: FlavorMinIO, Region: "us-east-1"},
			wantPathStyle:       true,
			wantDisableChecksum: true,
			wantEffectiveRegion: "us-east-1",
		},
		{
			name:                "Spaces uses virtual-hosted addressing",
			config:              &Config{Flavor: FlavorSpaces, Region: "nyc3"},
			wantPathStyle:       false,
			wantEffectiveRegion: "nyc3",
		},
		{
			name:                "B2 uses path-style and disables checksum",
			config:              &Config{Flavor: FlavorB2, Region: "us-west-004"},
			wantPathStyle:       true,
			wantDisableChecksum: true,
			wantEffectiveRegion: "us-west-004",
		},
		{
			name:                "unknown flavor falls back to generic",
			config:              &Config{Flavor: Flavor("wasabi"), Region: "us-east-2"},
			wantPathStyle:       true,
			wantEffectiveRegion: "us-east-2",
		},
		{
			name:                "explicit UsePathStyle override wins over flavor default",
			config:              &Config{Flavor: FlavorSpaces, Region: "nyc3", UsePathStyle: boolPtr(true)},
			wantPathStyle:       true,
			wantEffectiveRegion: "nyc3",
		},
		{
			name:                "explicit UsePathStyle can disable path-style on AWS",
			config:              &Config{Region: "eu-west-1", UsePathStyle: boolPtr(false)},
			wantPathStyle:       false,
			wantEffectiveRegion: "eu-west-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := resolveProfile(tt.config)

			assert.Equal(t, tt.wantPathStyle, p.usePathStyle, "usePathStyle")
			assert.Equal(t, tt.wantSigningRegion, p.signingRegion, "signingRegion")
			assert.Equal(t, tt.wantDisableChecksum, p.disableUploadChecksum, "disableUploadChecksum")
			assert.Equal(t, tt.wantEffectiveRegion, p.region(tt.config), "effective region")
		})
	}
}
