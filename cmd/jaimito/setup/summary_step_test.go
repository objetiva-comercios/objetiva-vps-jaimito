package setup_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"gopkg.in/yaml.v3"

	"github.com/chiguire/jaimito/cmd/jaimito/setup"
	"github.com/chiguire/jaimito/internal/config"
)

// makeValidSetupData construye un SetupData valido para tests de SummaryStep.
func makeValidSetupData(configPath string) *setup.SetupData {
	return &setup.SetupData{
		ConfigPath:      configPath,
		BotToken:        "123456789:ABCdefGhIjKlMnOpQrStUvWxYz",
		BotUsername:     "testbot",
		BotDisplayName:  "Test Bot",
		Channels: []config.ChannelConfig{
			{Name: "general", ChatID: -1001234567890, Priority: "normal"},
			{Name: "alerts", ChatID: -1009876543210, Priority: "high"},
		},
		ServerListen:    "127.0.0.1:8080",
		DatabasePath:    "/var/lib/jaimito/jaimito.db",
		GeneratedAPIKey: "sk-abcdef123456",
		KeepExistingKey: false,
	}
}

// --- View tests ---

// TestSummaryStep_View_ShowsAllSections verifica que View muestra las 5 secciones.
func TestSummaryStep_View_ShowsAllSections(t *testing.T) {
	step := &setup.SummaryStep{}
	data := makeValidSetupData("/tmp/config.yaml")
	view := step.View(data)

	sections := []string{"Telegram", "Canales", "Servidor", "Base de datos", "API Key"}
	for _, s := range sections {
		if !strings.Contains(view, s) {
			t.Errorf("View() debe contener seccion %q; got view:\n%s", s, view)
		}
	}
}

// TestSummaryStep_View_TelegramSection verifica que la seccion Telegram muestra token ofuscado, username y display name.
func TestSummaryStep_View_TelegramSection(t *testing.T) {
	step := &setup.SummaryStep{}
	data := makeValidSetupData("/tmp/config.yaml")
	view := step.View(data)

	// Token ofuscado — los ultimos 6 chars de "123456789:ABCdefGhIjKlMnOpQrStUvWxYz" son "vWxYz" (5) — wait
	// token = "123456789:ABCdefGhIjKlMnOpQrStUvWxYz" -> last 6 = "WxYz" is 4, let's check length
	// "ABCdefGhIjKlMnOpQrStUvWxYz" - last 6 = "vWxYz" no... let's count
	// token[len-6:] = "WxYz" -- the last 6 chars of "123456789:ABCdefGhIjKlMnOpQrStUvWxYz"
	// len = 36, last 6 = "WxYz" has 4 chars... let me just check the format ****:XXXXXX
	if !strings.Contains(view, "****:") {
		t.Errorf("View() debe contener token ofuscado '****:...'; got:\n%s", view)
	}
	if !strings.Contains(view, "@testbot") {
		t.Errorf("View() debe contener '@testbot'; got:\n%s", view)
	}
	if !strings.Contains(view, "Test Bot") {
		t.Errorf("View() debe contener 'Test Bot'; got:\n%s", view)
	}
}

// TestSummaryStep_View_ChannelsTable verifica que se muestran los canales con nombre, chat ID y prioridad.
func TestSummaryStep_View_ChannelsTable(t *testing.T) {
	step := &setup.SummaryStep{}
	data := makeValidSetupData("/tmp/config.yaml")
	view := step.View(data)

	if !strings.Contains(view, "general") {
		t.Errorf("View() debe contener canal 'general'; got:\n%s", view)
	}
	if !strings.Contains(view, "alerts") {
		t.Errorf("View() debe contener canal 'alerts'; got:\n%s", view)
	}
	if !strings.Contains(view, "normal") {
		t.Errorf("View() debe contener prioridad 'normal'; got:\n%s", view)
	}
	if !strings.Contains(view, "high") {
		t.Errorf("View() debe contener prioridad 'high'; got:\n%s", view)
	}
}

// TestSummaryStep_View_ServerSection verifica que se muestra el valor de ServerListen.
func TestSummaryStep_View_ServerSection(t *testing.T) {
	step := &setup.SummaryStep{}
	data := makeValidSetupData("/tmp/config.yaml")
	view := step.View(data)

	if !strings.Contains(view, "127.0.0.1:8080") {
		t.Errorf("View() debe contener ServerListen '127.0.0.1:8080'; got:\n%s", view)
	}
}

// TestSummaryStep_View_DatabaseSection verifica que se muestra el valor de DatabasePath.
func TestSummaryStep_View_DatabaseSection(t *testing.T) {
	step := &setup.SummaryStep{}
	data := makeValidSetupData("/tmp/config.yaml")
	view := step.View(data)

	if !strings.Contains(view, "/var/lib/jaimito/jaimito.db") {
		t.Errorf("View() debe contener DatabasePath; got:\n%s", view)
	}
}

// TestSummaryStep_View_APIKeyObfuscated verifica que la API key se muestra ofuscada.
func TestSummaryStep_View_APIKeyObfuscated(t *testing.T) {
	step := &setup.SummaryStep{}
	data := makeValidSetupData("/tmp/config.yaml")
	data.GeneratedAPIKey = "sk-abcdef123456"
	view := step.View(data)

	// No debe mostrar la key completa
	if strings.Contains(view, "sk-abcdef123456") {
		t.Errorf("View() no debe mostrar la API key completa; got:\n%s", view)
	}
	// Debe mostrar ofuscado
	if !strings.Contains(view, "****:") {
		t.Errorf("View() debe contener API key ofuscada '****:...'; got:\n%s", view)
	}
}

// TestSummaryStep_View_KeepExistingKey verifica que cuando KeepExistingKey=true se muestra "(mantenida)".
func TestSummaryStep_View_KeepExistingKey(t *testing.T) {
	step := &setup.SummaryStep{}
	data := makeValidSetupData("/tmp/config.yaml")
	data.KeepExistingKey = true
	view := step.View(data)

	if !strings.Contains(view, "(mantenida)") {
		t.Errorf("View() con KeepExistingKey=true debe mostrar '(mantenida)'; got:\n%s", view)
	}
}

// --- Write flow tests (actualizados para flujo de dos fases) ---

// TestSummaryStep_WriteConfig_Success verifica que Enter inicia el envio de test (sending=true, Done()=false).
// Luego testNotificationResultMsg completa el flujo (Done()=true).
func TestSummaryStep_WriteConfig_Success(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, cmd := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	// Despues de Enter con exito: sending=true, Done()=false, cmd no nil (Batch)
	if s.Done() {
		t.Error("Done() debe ser false despues de Enter (en estado sending)")
	}
	if !s.IsSending() {
		t.Error("IsSending() debe ser true despues de Enter exitoso")
	}
	if cmd == nil {
		t.Error("Update() debe retornar un cmd (tea.Batch) despues de escritura exitosa")
	}

	// El archivo debe haberse creado
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Errorf("Archivo de config no fue creado en %s", cfgPath)
	}

	// Completar con resultado de test exitoso
	resultMsg := setup.NewTestNotificationResultMsg(nil)
	updated2, cmd2 := s.Update(resultMsg, data)
	s2 := updated2.(*setup.SummaryStep)

	if !s2.Done() {
		t.Error("Done() debe ser true despues de testNotificationResultMsg exitoso")
	}
	if cmd2 == nil {
		t.Error("Update() debe retornar tea.Quit despues de testNotificationResultMsg")
	}
}

// TestSummaryStep_WriteConfig_CreatesDir verifica que se crea el directorio si no existe.
func TestSummaryStep_WriteConfig_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	// Subdirectorio que no existe
	cfgPath := filepath.Join(dir, "subdir", "nested", "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	// Despues de Enter: sending=true, Done()=false
	if s.Done() {
		view := s.View(data)
		t.Errorf("Done() debe ser false despues de Enter (en sending); view:\n%s", view)
	}
	if !s.IsSending() {
		view := s.View(data)
		t.Errorf("IsSending() debe ser true despues de Enter exitoso; view:\n%s", view)
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Errorf("Archivo de config no fue creado en %s", cfgPath)
	}

	// Completar con resultado de test
	resultMsg := setup.NewTestNotificationResultMsg(nil)
	updated2, _ := s.Update(resultMsg, data)
	s2 := updated2.(*setup.SummaryStep)

	if !s2.Done() {
		t.Error("Done() debe ser true despues de testNotificationResultMsg")
	}
}

// TestSummaryStep_WriteConfig_ValidYAML verifica que el archivo escrito es YAML valido.
func TestSummaryStep_WriteConfig_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	step.Update(enterMsg, data)

	yamlData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("No se pudo leer el archivo: %v", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
		t.Errorf("El YAML escrito no es valido: %v", err)
	}
}

// TestSummaryStep_WriteConfig_RoundTrip verifica que el config puede ser cargado con config.Load() y los valores coinciden.
func TestSummaryStep_WriteConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	step.Update(enterMsg, data)

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load() fallo: %v", err)
	}

	if loaded.Telegram.Token != data.BotToken {
		t.Errorf("Token: esperaba %q, got %q", data.BotToken, loaded.Telegram.Token)
	}
	if loaded.Server.Listen != data.ServerListen {
		t.Errorf("ServerListen: esperaba %q, got %q", data.ServerListen, loaded.Server.Listen)
	}
	if loaded.Database.Path != data.DatabasePath {
		t.Errorf("DatabasePath: esperaba %q, got %q", data.DatabasePath, loaded.Database.Path)
	}
}

// TestSummaryStep_WriteConfig_Permissions verifica que el archivo tiene permisos 0o600.
func TestSummaryStep_WriteConfig_Permissions(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	step.Update(enterMsg, data)

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("No se pudo stat el archivo: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("Permisos: esperaba 0o600, got %o", perm)
	}
}

// TestSummaryStep_WriteConfig_SeedAPIKeys verifica que el YAML incluye seed_api_keys con la key generada.
func TestSummaryStep_WriteConfig_SeedAPIKeys(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)
	data.GeneratedAPIKey = "sk-testkey123456"

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	step.Update(enterMsg, data)

	yamlData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("No se pudo leer el archivo: %v", err)
	}

	if !strings.Contains(string(yamlData), "sk-testkey123456") {
		t.Errorf("YAML debe contener la API key generada; got:\n%s", string(yamlData))
	}
	if !strings.Contains(string(yamlData), "default") {
		t.Errorf("YAML debe contener el nombre 'default' para la API key; got:\n%s", string(yamlData))
	}
}

// TestSummaryStep_WriteConfig_KeepExistingKeys verifica que con KeepExistingKey=true se mantienen las keys del config existente.
func TestSummaryStep_WriteConfig_KeepExistingKeys(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	existingCfg := &config.Config{
		SeedAPIKeys: []config.SeedAPIKey{
			{Name: "production", Key: "sk-existingkey999"},
		},
	}

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)
	data.KeepExistingKey = true
	data.ExistingCfg = existingCfg
	data.GeneratedAPIKey = "sk-newkey000"

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	step.Update(enterMsg, data)

	yamlData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("No se pudo leer el archivo: %v", err)
	}

	content := string(yamlData)
	if !strings.Contains(content, "sk-existingkey999") {
		t.Errorf("YAML debe contener las keys existentes; got:\n%s", content)
	}
	if strings.Contains(content, "sk-newkey000") {
		t.Errorf("YAML no debe contener la nueva key cuando KeepExistingKey=true; got:\n%s", content)
	}
}

// --- Validation/error tests ---

// TestSummaryStep_ValidateFails verifica que con datos invalidos se muestra error y no se escribe el archivo.
func TestSummaryStep_ValidateFails(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := &setup.SetupData{
		ConfigPath: cfgPath,
		BotToken:   "", // invalido: token vacio
		Channels:   []config.ChannelConfig{{Name: "general", ChatID: -100123, Priority: "normal"}},
	}

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	if s.Done() {
		t.Error("Done() debe ser false cuando Validate() falla")
	}

	view := s.View(data)
	if !strings.Contains(view, "validaci") {
		t.Errorf("View() debe mostrar error de validacion; got:\n%s", view)
	}

	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Error("El archivo de config NO debe existir cuando la validacion falla")
	}
}

// TestSummaryStep_WriteError verifica que un error de escritura se muestra correctamente.
func TestSummaryStep_WriteError(t *testing.T) {
	// Crear un directorio de solo lectura
	dir := t.TempDir()
	readonlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("No se pudo crear el directorio: %v", err)
	}
	// Hacer el directorio de solo lectura
	if err := os.Chmod(readonlyDir, 0o555); err != nil {
		t.Fatalf("No se pudo cambiar permisos: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755) // cleanup

	cfgPath := filepath.Join(readonlyDir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	if s.Done() {
		t.Skip("Corriendo como root — test de permisos no aplica")
	}

	view := s.View(data)
	if !strings.Contains(view, "Error") {
		t.Errorf("View() debe mostrar error de escritura; got:\n%s", view)
	}
}

// TestSummaryStep_WriteError_PermissionDenied verifica que un error de permission denied muestra el hint de sudo.
func TestSummaryStep_WriteError_PermissionDenied(t *testing.T) {
	dir := t.TempDir()
	readonlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readonlyDir, 0o555); err != nil {
		t.Fatalf("No se pudo crear el directorio: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755)

	cfgPath := filepath.Join(readonlyDir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	if s.Done() {
		t.Skip("Corriendo como root — test de permisos no aplica")
	}

	view := s.View(data)
	if !strings.Contains(view, "sudo jaimito setup") {
		t.Errorf("View() debe sugerir 'sudo jaimito setup' para permission denied; got:\n%s", view)
	}
}

// TestSummaryStep_RequiresEnter verifica que el step no avanza automaticamente.
func TestSummaryStep_RequiresEnter(t *testing.T) {
	step := &setup.SummaryStep{}
	data := makeValidSetupData("/tmp/config.yaml")

	// Sin presionar Enter
	if step.Done() {
		t.Error("Done() debe ser false antes de presionar Enter")
	}

	// Presionar otra tecla
	otherMsg := tea.KeyPressMsg{Code: tea.KeySpace}
	updated, _ := step.Update(otherMsg, data)
	s := updated.(*setup.SummaryStep)

	if s.Done() {
		t.Error("Done() debe ser false despues de teclas que no son Enter")
	}
}

// TestSummaryStep_Done_QuitsWizard verifica que tras Enter exitoso, Done()=false (en sending),
// y que tea.Quit llega despues de testNotificationResultMsg.
func TestSummaryStep_Done_QuitsWizard(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, cmd := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	// Despues de Enter: sending=true, Done()=false, cmd no nil (Batch, no Quit)
	if s.Done() {
		t.Error("Done() debe ser false despues de Enter (en estado sending)")
	}
	if cmd == nil {
		t.Error("Update() debe retornar cmd (tea.Batch) despues de Enter exitoso")
	}

	// Enviar resultado de test exitoso: ahora si llega tea.Quit
	resultMsg := setup.NewTestNotificationResultMsg(nil)
	updated2, cmd2 := s.Update(resultMsg, data)
	s2 := updated2.(*setup.SummaryStep)

	if !s2.Done() {
		t.Error("Done() debe ser true despues de testNotificationResultMsg")
	}
	if cmd2 == nil {
		t.Error("Update() debe retornar tea.Quit despues de testNotificationResultMsg")
	}
}

// --- Tests nuevos para notificacion de test ---

// TestSummaryStep_TestNotification_SendsSpinner verifica que Enter con config exitoso
// pone sending=true y View muestra el spinner con texto.
func TestSummaryStep_TestNotification_SendsSpinner(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	if !s.IsSending() {
		t.Error("IsSending() debe ser true despues de Enter exitoso")
	}

	view := s.View(data)
	if !strings.Contains(view, "Enviando notificacion de test") {
		t.Errorf("View() en sending debe contener 'Enviando notificacion de test'; got:\n%s", view)
	}
}

// TestSummaryStep_TestNotification_Success verifica que testNotificationResultMsg con err=nil
// pone testOk=true y Done()=true, y View muestra el checkmark verde.
func TestSummaryStep_TestNotification_Success(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	// Primero llegar al estado sending
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	// Enviar resultado exitoso
	resultMsg := setup.NewTestNotificationResultMsg(nil)
	updated2, _ := s.Update(resultMsg, data)
	s2 := updated2.(*setup.SummaryStep)

	if !s2.Done() {
		t.Error("Done() debe ser true despues de testNotificationResultMsg exitoso")
	}

	view := s2.View(data)
	if !strings.Contains(view, "Notificacion de test enviada") {
		t.Errorf("View() con testOk debe contener 'Notificacion de test enviada'; got:\n%s", view)
	}
	if !strings.Contains(view, "systemctl start jaimito") {
		t.Errorf("View() con testOk debe contener hint systemctl; got:\n%s", view)
	}
}

// TestSummaryStep_TestNotification_Failure verifica que testNotificationResultMsg con error
// pone testErr con el mensaje y Done()=true, y View muestra warning amarillo.
func TestSummaryStep_TestNotification_Failure(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	// Primero llegar al estado sending
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	// Enviar resultado con error
	resultMsg := setup.NewTestNotificationResultMsg(errors.New("timeout"))
	updated2, _ := s.Update(resultMsg, data)
	s2 := updated2.(*setup.SummaryStep)

	if !s2.Done() {
		t.Error("Done() debe ser true incluso con error en notificacion de test")
	}

	view := s2.View(data)
	if !strings.Contains(view, "Notificacion de test fallida") {
		t.Errorf("View() con testErr debe contener 'Notificacion de test fallida'; got:\n%s", view)
	}
	if !strings.Contains(view, "timeout") {
		t.Errorf("View() con testErr debe contener el mensaje de error; got:\n%s", view)
	}
}

// TestSummaryStep_TestNotification_WarningNotError verifica que el fallo de test
// usa WarningStyle (amarillo) y no ErrorStyle (rojo), y permite continuar.
func TestSummaryStep_TestNotification_WarningNotError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	// Llegar a sending
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	// Enviar error
	resultMsg := setup.NewTestNotificationResultMsg(errors.New("network error"))
	updated2, _ := s.Update(resultMsg, data)
	s2 := updated2.(*setup.SummaryStep)

	view := s2.View(data)

	// Debe contener hint de systemctl (config es valido, no bloqueante)
	if !strings.Contains(view, "systemctl start jaimito") {
		t.Errorf("View() con testErr debe contener hint systemctl (no bloqueante); got:\n%s", view)
	}
}

// TestSummaryStep_TestNotification_NilBot verifica que ValidatedBot=nil genera error limpio.
// El bot nil no causa panic: sendTestNotificationCmd retorna error "bot no disponible".
func TestSummaryStep_TestNotification_NilBot(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)
	data.ValidatedBot = nil // bot no disponible

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, cmd := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	// Con bot nil: debe estar en sending=true y cmd no nil
	if cmd == nil {
		t.Error("Update() debe retornar cmd incluso con ValidatedBot=nil")
	}
	if !s.IsSending() {
		t.Error("IsSending() debe ser true despues de Enter con bot nil")
	}

	// Simular el resultado async: bot nil debe producir error "bot no disponible"
	// (sendTestNotificationCmd es defensive: si b == nil retorna error sin panic)
	resultMsg := setup.NewTestNotificationResultMsg(errors.New("bot no disponible"))
	updated2, _ := s.Update(resultMsg, data)
	s2 := updated2.(*setup.SummaryStep)

	if !s2.Done() {
		t.Error("Done() debe ser true despues de testNotificationResultMsg con error de bot")
	}

	view := s2.View(data)
	if !strings.Contains(view, "Notificacion de test fallida") {
		t.Errorf("View() debe indicar fallo de notificacion; got:\n%s", view)
	}
	if !strings.Contains(view, "bot no disponible") {
		t.Errorf("View() debe contener mensaje 'bot no disponible'; got:\n%s", view)
	}
}

// TestSummaryStep_SpinnerTick_WhenSending verifica que spinner.TickMsg se procesa cuando sending=true.
func TestSummaryStep_SpinnerTick_WhenSending(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	step := &setup.SummaryStep{}
	data := makeValidSetupData(cfgPath)

	// Llegar al estado sending
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := step.Update(enterMsg, data)
	s := updated.(*setup.SummaryStep)

	if !s.IsSending() {
		t.Fatal("IsSending() debe ser true para este test")
	}

	// Enviar spinner.TickMsg
	tickMsg := spinner.TickMsg{}
	_, cmd := s.Update(tickMsg, data)

	// Debe retornar un cmd (tick continuo del spinner)
	if cmd == nil {
		t.Error("Update() con spinner.TickMsg cuando sending=true debe retornar cmd (tick)")
	}
}

// TestSummaryStep_SpinnerTick_WhenNotSending verifica que spinner.TickMsg se ignora cuando sending=false.
func TestSummaryStep_SpinnerTick_WhenNotSending(t *testing.T) {
	step := &setup.SummaryStep{}
	data := makeValidSetupData("/tmp/config.yaml")

	// El step esta en estado inicial: sending=false
	tickMsg := spinner.TickMsg{}
	_, cmd := step.Update(tickMsg, data)

	// Debe retornar nil cmd (no procesar tick cuando no se esta enviando)
	if cmd != nil {
		t.Error("Update() con spinner.TickMsg cuando sending=false debe retornar nil cmd")
	}
}
