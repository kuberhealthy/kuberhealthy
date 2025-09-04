package main

import (
	"net/http"

	ui "github.com/kuberhealthy/kuberhealthy/v3/assets/ui"
)

var uiFS = http.FS(ui.Dist)

// statusPageHandler serves the Svelte UI.
func statusPageHandler(w http.ResponseWriter, r *http.Request) error {
	http.FileServer(uiFS).ServeHTTP(w, r)
	return nil
}
