//go:build prod

package server

import "embed"

//go:embed all:assets
var staticFS embed.FS