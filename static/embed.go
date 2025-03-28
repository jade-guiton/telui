package static

import "embed"

//go:embed index.html icon.png *.js *.css
var StaticFs embed.FS
