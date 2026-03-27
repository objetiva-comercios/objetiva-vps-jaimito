// Package collector implements the metrics collection loop for jaimito.
// It executes shell commands at configurable intervals and persists readings to SQLite.
package collector

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// computeTimeout calcula el timeout para ejecutar un comando de metrica.
// El timeout es el 80% del intervalo de la metrica, con un maximo de 30 segundos.
// Por D-05: timeout = min(80%*interval, 30s).
func computeTimeout(interval time.Duration) time.Duration {
	timeout := time.Duration(float64(interval) * 0.8)
	if timeout > 30*time.Second {
		return 30 * time.Second
	}
	return timeout
}

// runCommand ejecuta un comando shell con timeout y retorna el valor float64 del output.
// Usa "sh -c" (POSIX sh, no bash) para maxima portabilidad en Linux.
// Por D-04: exec.CommandContext con timeout + WaitDelay para forzar kill.
// Retorna 0 y error si el comando falla, excede el timeout, o el output no es numerico.
func runCommand(ctx context.Context, command string, timeout time.Duration) (float64, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", command)
	// WaitDelay fuerza el kill del proceso despues del timeout del contexto.
	// Esto evita goroutine leaks cuando el proceso hijo no responde a SIGTERM.
	cmd.WaitDelay = 5 * time.Second

	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("command failed: %w", err)
	}

	trimmed := strings.TrimSpace(string(out))
	val, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("parse output %q: %w", trimmed, err)
	}

	return val, nil
}
