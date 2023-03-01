package embeddedassets

import (
	"embed"
)

//go:embed files
var Assets embed.FS

const Root = "files"
