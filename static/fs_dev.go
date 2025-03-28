//go:build dev

package static

import (
	"os"
)

var StaticFs = os.DirFS("static")
