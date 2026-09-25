// Package apidocs sirve la documentación de la API: la especificación OpenAPI y la interfaz Swagger UI.
// Todo va embebido en el binario (sin CDN), así la documentación funciona sin internet.
package apidocs

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed openapi.yaml
var spec []byte

//go:embed ui
var uiFS embed.FS

// Register monta /openapi.yaml y la interfaz en /docs/. /docs redirige a /docs/ para que las rutas
// relativas de los archivos estáticos funcionen.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(spec)
	})
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "docs/", http.StatusMovedPermanently)
	})
	ui, _ := fs.Sub(uiFS, "ui")
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServerFS(ui)))
}
