package setup

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chiguire/jaimito/internal/config"
)

// TestDetectConfig_ValidConfig verifica que con un config valido se muestra
// el token ofuscado, la cantidad de canales, la direccion del servidor, y las 3 opciones.
func TestDetectConfig_ValidConfig(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{Token: "123456:ABCDEFghijklmnop"},
		Channels: []config.ChannelConfig{
			{Name: "general", ChatID: 100, Priority: "normal"},
			{Name: "alerts", ChatID: 200, Priority: "high"},
		},
		Server:   config.ServerConfig{Listen: "127.0.0.1:8080"},
		Database: config.DatabaseConfig{Path: "/var/lib/jaimito/jaimito.db"},
	}

	data := &SetupData{
		ConfigPath:    "/etc/jaimito/config.yaml",
		ExistingCfg:   cfg,
		ConfigErr:     nil,
		ConfigExists:  true,
	}

	step := &DetectConfigStep{}
	step.Init(data)
	view := step.View(data)

	// Token ofuscado: ultimos 6 chars de "123456:ABCDEFghijklmnop" = "klmnop"
	if !strings.Contains(view, "****:klmnop") {
		t.Errorf("View() debe contener '****:klmnop'; got: %q", view)
	}
	// Cantidad de canales
	if !strings.Contains(view, "2") {
		t.Errorf("View() debe contener '2' (canales); got: %q", view)
	}
	// Listen address
	if !strings.Contains(view, "127.0.0.1:8080") {
		t.Errorf("View() debe contener '127.0.0.1:8080'; got: %q", view)
	}
	// 3 opciones
	if !strings.Contains(view, "Editar") {
		t.Errorf("View() debe contener 'Editar'; got: %q", view)
	}
	if !strings.Contains(view, "Crear desde cero") {
		t.Errorf("View() debe contener 'Crear desde cero'; got: %q", view)
	}
	if !strings.Contains(view, "Cancelar") {
		t.Errorf("View() debe contener 'Cancelar'; got: %q", view)
	}
}

// TestDetectConfig_InvalidConfig verifica que con config invalido se muestra
// el error especifico y solo 2 opciones (sin Editar).
func TestDetectConfig_InvalidConfig(t *testing.T) {
	// Crear un archivo temporal para simular que el config existe pero es invalido
	tmpFile, err := os.CreateTemp("", "jaimito-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	data := &SetupData{
		ConfigPath:   tmpFile.Name(),
		ExistingCfg:  nil,
		ConfigErr:    fmt.Errorf("telegram.token is required"),
		ConfigExists: true,
	}

	step := &DetectConfigStep{}
	step.Init(data)
	view := step.View(data)

	// Debe contener el error especifico
	if !strings.Contains(view, "telegram.token is required") {
		t.Errorf("View() debe contener el error; got: %q", view)
	}
	// Debe contener Crear desde cero y Cancelar
	if !strings.Contains(view, "Crear desde cero") {
		t.Errorf("View() debe contener 'Crear desde cero'; got: %q", view)
	}
	if !strings.Contains(view, "Cancelar") {
		t.Errorf("View() debe contener 'Cancelar'; got: %q", view)
	}
	// NO debe contener Editar
	if strings.Contains(view, "Editar") {
		t.Errorf("View() NO debe contener 'Editar' para config invalido; got: %q", view)
	}
}

// TestDetectConfig_MissingConfig verifica que si el config no existe,
// Done() retorna true inmediatamente y Mode="new".
func TestDetectConfig_MissingConfig(t *testing.T) {
	data := &SetupData{
		ConfigPath:   "/no/existe/config.yaml",
		ExistingCfg:  nil,
		ConfigErr:    nil,
		ConfigExists: false,
	}

	step := &DetectConfigStep{}
	step.Init(data)

	if !step.Done() {
		t.Error("Done() debe ser true cuando el config no existe (skip step)")
	}
	if data.Mode != "new" {
		t.Errorf("data.Mode debe ser 'new' cuando el config no existe; got: %q", data.Mode)
	}
}

// TestObfuscateToken verifica la ofuscacion correcta del token.
func TestObfuscateToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123456:ABCDEFghijklmnop", "****:klmnop"},
		{"short", "****"},
		{"", "****"},
		{"123456", "****"},   // exactamente 6 chars -> "****"
		{"1234567", "****:234567"}, // 7 chars -> last 6 = "234567"
	}
	for _, tt := range tests {
		got := obfuscateToken(tt.input)
		if got != tt.want {
			t.Errorf("obfuscateToken(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

// TestDetectConfig_SelectEdit verifica que seleccionar "Editar" setea Mode="edit" y Done()=true.
func TestDetectConfig_SelectEdit(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{Token: "123456:ABCDEFghijklmnop"},
		Channels: []config.ChannelConfig{
			{Name: "general", ChatID: 100, Priority: "normal"},
		},
		Server:   config.ServerConfig{Listen: "127.0.0.1:8080"},
		Database: config.DatabaseConfig{Path: "/var/lib/jaimito/jaimito.db"},
	}

	data := &SetupData{
		ConfigPath:   "/etc/jaimito/config.yaml",
		ExistingCfg:  cfg,
		ConfigErr:    nil,
		ConfigExists: true,
	}

	step := &DetectConfigStep{}
	step.Init(data)

	// Simular seleccion de opcion 0 (Editar) con Enter
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updatedStep, _ := step.Update(enterMsg, data)

	if !updatedStep.Done() {
		t.Error("Done() debe ser true despues de seleccionar Editar")
	}
	if data.Mode != "edit" {
		t.Errorf("data.Mode debe ser 'edit'; got: %q", data.Mode)
	}
}

// TestDetectConfig_SelectFresh verifica que seleccionar "Crear desde cero" setea Mode="fresh" y Done()=true.
func TestDetectConfig_SelectFresh(t *testing.T) {
	// Crear archivo temporal para backup
	tmpFile, err := os.CreateTemp("", "jaimito-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.WriteString("token: abc\n")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	defer os.Remove(tmpFile.Name() + ".bak")

	cfg := &config.Config{
		Telegram: config.TelegramConfig{Token: "123456:ABCDEFghijklmnop"},
		Channels: []config.ChannelConfig{
			{Name: "general", ChatID: 100, Priority: "normal"},
		},
		Server:   config.ServerConfig{Listen: "127.0.0.1:8080"},
		Database: config.DatabaseConfig{Path: "/var/lib/jaimito/jaimito.db"},
	}

	data := &SetupData{
		ConfigPath:   tmpFile.Name(),
		ExistingCfg:  cfg,
		ConfigErr:    nil,
		ConfigExists: true,
	}

	step := &DetectConfigStep{}
	step.Init(data)

	// Mover seleccion a indice 1 (Crear desde cero) con flecha abajo
	downMsg := tea.KeyPressMsg{Code: tea.KeyDown}
	step2, _ := step.Update(downMsg, data)

	// Seleccionar con Enter
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updatedStep, _ := step2.Update(enterMsg, data)

	if !updatedStep.Done() {
		t.Error("Done() debe ser true despues de seleccionar Crear desde cero")
	}
	if data.Mode != "fresh" {
		t.Errorf("data.Mode debe ser 'fresh'; got: %q", data.Mode)
	}
}

// TestDetectConfig_SelectCancel verifica que seleccionar "Cancelar" retorna tea.Quit.
func TestDetectConfig_SelectCancel(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{Token: "123456:ABCDEFghijklmnop"},
		Channels: []config.ChannelConfig{
			{Name: "general", ChatID: 100, Priority: "normal"},
		},
		Server:   config.ServerConfig{Listen: "127.0.0.1:8080"},
		Database: config.DatabaseConfig{Path: "/var/lib/jaimito/jaimito.db"},
	}

	data := &SetupData{
		ConfigPath:   "/etc/jaimito/config.yaml",
		ExistingCfg:  cfg,
		ConfigErr:    nil,
		ConfigExists: true,
	}

	step := &DetectConfigStep{}
	step.Init(data)

	// Mover a indice 2 (Cancelar) con dos flechas abajo
	downMsg := tea.KeyPressMsg{Code: tea.KeyDown}
	step2, _ := step.Update(downMsg, data)
	step3, _ := step2.Update(downMsg, data)

	// Seleccionar con Enter
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, cmd := step3.Update(enterMsg, data)

	if cmd == nil {
		t.Fatal("Seleccionar Cancelar debe retornar un comando")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("Cancelar debe producir tea.QuitMsg; got: %T", msg)
	}
}

// TestBackupConfig verifica que backupConfig copia el archivo a path+".bak".
func TestBackupConfig(t *testing.T) {
	// Crear archivo temporal con contenido conocido
	tmpFile, err := os.CreateTemp("", "jaimito-backup-test-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	content := "telegram:\n  token: abc123\n"
	tmpFile.WriteString(content)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	defer os.Remove(tmpFile.Name() + ".bak")

	// Llamar backupConfig
	if err := backupConfig(tmpFile.Name()); err != nil {
		t.Fatalf("backupConfig() error: %v", err)
	}

	// Verificar que el .bak existe
	bakPath := tmpFile.Name() + ".bak"
	bakData, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("No se pudo leer el backup: %v", err)
	}
	if string(bakData) != content {
		t.Errorf("Contenido del backup incorrecto; got: %q; want: %q", string(bakData), content)
	}
}
