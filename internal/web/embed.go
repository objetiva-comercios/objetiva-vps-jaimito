package web

import (
	"bytes"
	"embed"
	"net/http"
	"os"
	"sync"
)

//go:embed index.html
var webFS embed.FS

// hostname caches os.Hostname() at first call (per Pitfall 1 in RESEARCH.md).
var hostname = sync.OnceValue(func() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
})

// DashboardHandler retorna un http.HandlerFunc que sirve index.html embedido.
// Reemplaza el placeholder {{HOSTNAME}} con el hostname real del VPS.
// Sin auth, siguiendo el patron de HealthHandler (per D-14).
func DashboardHandler() http.HandlerFunc {
	// Read the embedded HTML once at handler creation time.
	rawHTML, _ := webFS.ReadFile("index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		h := hostname()
		out := bytes.ReplaceAll(rawHTML, []byte("{{HOSTNAME}}"), []byte(h))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(out)
	}
}
