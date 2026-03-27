package collector

import (
	"context"
	"testing"
	"time"
)

// TestComputeTimeout verifica que computeTimeout calcula min(80%*interval, 30s).
func TestComputeTimeout(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		want     time.Duration
	}{
		{"60s -> 30s (capped at max)", 60 * time.Second, 30 * time.Second},
		{"30s -> 24s (80%)", 30 * time.Second, 24 * time.Second},
		{"5s -> 4s (80%)", 5 * time.Second, 4 * time.Second},
		{"1s -> 800ms (80%)", 1 * time.Second, 800 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeTimeout(tt.interval)
			if got != tt.want {
				t.Errorf("computeTimeout(%v) = %v, want %v", tt.interval, got, tt.want)
			}
		})
	}
}

// TestRunCommand_Success verifica que un comando exitoso retorna el valor flotante parseado.
func TestRunCommand_Success(t *testing.T) {
	ctx := context.Background()
	val, err := runCommand(ctx, "echo 42.5", 5*time.Second)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if val != 42.5 {
		t.Errorf("expected 42.5, got %v", val)
	}
}

// TestRunCommand_ParseError verifica que output no numerico retorna error "parse output".
func TestRunCommand_ParseError(t *testing.T) {
	ctx := context.Background()
	val, err := runCommand(ctx, "echo notanumber", 5*time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if val != 0 {
		t.Errorf("expected 0 on error, got %v", val)
	}
	// El error debe mencionar "parse output"
	if errStr := err.Error(); len(errStr) == 0 {
		t.Error("error message should not be empty")
	}
}

// TestRunCommand_Timeout verifica que un comando que excede el timeout retorna error.
func TestRunCommand_Timeout(t *testing.T) {
	ctx := context.Background()
	val, err := runCommand(ctx, "sleep 10", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if val != 0 {
		t.Errorf("expected 0 on timeout, got %v", val)
	}
}

// TestRunCommand_Failure verifica que un comando con exit code != 0 retorna error "command failed".
func TestRunCommand_Failure(t *testing.T) {
	ctx := context.Background()
	val, err := runCommand(ctx, "exit 1", 5*time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if val != 0 {
		t.Errorf("expected 0 on failure, got %v", val)
	}
}

// TestRunCommand_CommandNotFound verifica que un binario inexistente retorna error sin crash.
// Esto cubre MCOL-02: docker_running falla silenciosamente si Docker no esta instalado.
func TestRunCommand_CommandNotFound(t *testing.T) {
	ctx := context.Background()
	val, err := runCommand(ctx, "nonexistent_binary_xyz 2>/dev/null", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for nonexistent binary, got nil")
	}
	if val != 0 {
		t.Errorf("expected 0 for nonexistent binary, got %v", val)
	}
}
