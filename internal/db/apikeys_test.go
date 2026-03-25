package db_test

import (
	"strings"
	"testing"

	"github.com/chiguire/jaimito/internal/db"
)

// TestGenerateRawKey_Format verifica que la key generada tiene el prefijo "sk-" y 67 chars totales.
func TestGenerateRawKey_Format(t *testing.T) {
	key, err := db.GenerateRawKey()
	if err != nil {
		t.Fatalf("GenerateRawKey() error inesperado: %v", err)
	}
	if !strings.HasPrefix(key, "sk-") {
		t.Errorf("GenerateRawKey() debe empezar con 'sk-'; got: %q", key)
	}
	// "sk-" (3) + 64 hex chars = 67 chars total
	if len(key) != 67 {
		t.Errorf("GenerateRawKey() debe tener 67 chars; got: %d (%q)", len(key), key)
	}
}

// TestGenerateRawKey_Unique verifica que dos llamadas producen keys distintas.
func TestGenerateRawKey_Unique(t *testing.T) {
	key1, err := db.GenerateRawKey()
	if err != nil {
		t.Fatalf("GenerateRawKey() primera llamada error: %v", err)
	}
	key2, err := db.GenerateRawKey()
	if err != nil {
		t.Fatalf("GenerateRawKey() segunda llamada error: %v", err)
	}
	if key1 == key2 {
		t.Errorf("GenerateRawKey() debe generar keys unicas; ambas: %q", key1)
	}
}
