package web

import (
	"embed"
	"net/http"
)

//go:embed index.html style.css app.js
var Assets embed.FS

// Handler returns the file server handler for embedded assets.
func Handler() http.Handler {
	return http.FileServer(http.FS(Assets))
}
