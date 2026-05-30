//go:build !prod

package server

import "io/fs"

// staticFS — stub dev : jamais utilisé quand UNYPORT_ASSETS est défini.
// Déclaré pour satisfaire le compilateur (routes.go référence staticFS).
var staticFS fs.FS