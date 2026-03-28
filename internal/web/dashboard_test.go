package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDashboardHandler verifica que DashboardHandler sirve HTML con hostname inyectado (DASH-01).
func TestDashboardHandler(t *testing.T) {
	handler := DashboardHandler()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected Content-Type text/html, got %s", ct)
	}
	body := w.Body.String()
	// El placeholder {{HOSTNAME}} debe haber sido reemplazado por el hostname real
	if strings.Contains(body, "{{HOSTNAME}}") {
		t.Error("{{HOSTNAME}} placeholder was not replaced")
	}
	// El HTML debe contener contenido basico
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("response does not contain <!DOCTYPE html>")
	}
}

// TestEmbedAssets verifica que el FS embedido contiene index.html (DASH-05).
func TestEmbedAssets(t *testing.T) {
	data, err := webFS.ReadFile("index.html")
	if err != nil {
		t.Fatalf("webFS.ReadFile(index.html) failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("index.html is empty")
	}
}

// TestHostnameInjection verifica que el hostname se inyecta correctamente.
func TestHostnameInjection(t *testing.T) {
	handler := DashboardHandler()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	// Debe contener el hostname real del sistema
	h := hostname()
	if h != "unknown" && !strings.Contains(body, h) {
		t.Errorf("expected hostname %q in response body", h)
	}
}
