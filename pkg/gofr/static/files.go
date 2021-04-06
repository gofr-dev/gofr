package static

import "embed"

//go:embed *

var Files embed.FS //nolint:gochecknoglobals // Go embed requires it to be a global variable.
