// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ExecResult resultado da execução de código
type ExecResult struct {
	Output   string        `json:"output"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration"`
	Language string        `json:"language"`
}

// Service executa código em ambiente sandboxed
type Service struct {
	workDir    string
	maxTimeout time.Duration
	useDocker  bool
}

// NewService cria sandbox com diretório de trabalho isolado.
// Detecta automaticamente se Docker está disponível para isolamento.
func NewService(workDir string) *Service {
	os.MkdirAll(workDir, 0755)

	// Verificar se Docker está disponível
	useDocker := false
	if _, err := exec.LookPath("docker"); err == nil {
		useDocker = true
	}

	return &Service{
		workDir:    workDir,
		maxTimeout: 2 * time.Minute,
		useDocker:  useDocker,
	}
}

// Execute executa código na linguagem especificada.
// Se Docker estiver disponível, executa dentro de container isolado.
// Caso contrário, faz fallback para execução direta.
func (s *Service) Execute(ctx context.Context, language, code string, timeout time.Duration) (*ExecResult, error) {
	if s.useDocker {
		return s.executeInDocker(ctx, language, code, timeout)
	}

	// Fallback: execução direta (sem Docker)
	return s.executeDirect(ctx, language, code, timeout)
}

// executeDirect executa código diretamente no host (fallback sem Docker)
func (s *Service) executeDirect(ctx context.Context, language, code string, timeout time.Duration) (*ExecResult, error) {
	switch strings.ToLower(language) {
	case "bash", "sh", "shell":
		return s.executeBash(ctx, code, timeout)
	case "python", "python3", "py":
		return s.executePython(ctx, code, timeout)
	case "node", "javascript", "js":
		return s.executeNode(ctx, code, timeout)
	default:
		return nil, fmt.Errorf("linguagem não suportada: %s (use bash, python ou node)", language)
	}
}

// executeInDocker executa código dentro de um container Docker isolado
func (s *Service) executeInDocker(ctx context.Context, language, code string, timeout time.Duration) (*ExecResult, error) {
	if timeout <= 0 || timeout > s.maxTimeout {
		timeout = s.maxTimeout
	}

	// Determinar imagem e comando baseado na linguagem
	var image, shell, flag string
	switch strings.ToLower(language) {
	case "bash", "sh", "shell":
		image = "alpine:3.20"
		shell = "sh"
		flag = "-c"
	case "python", "python3", "py":
		image = "python:3.12-slim"
		shell = "python"
		flag = "-c"
	case "node", "javascript", "js":
		image = "node:20-slim"
		shell = "node"
		flag = "-e"
	default:
		return nil, fmt.Errorf("linguagem não suportada: %s (use bash, python ou node)", language)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Security: bloquear comandos destrutivos para bash
	if strings.ToLower(language) == "bash" || strings.ToLower(language) == "sh" || strings.ToLower(language) == "shell" {
		normalized := strings.ToLower(code)
		dangerous := []string{"rm -rf /", "mkfs", "dd if=", ":(){", "fork bomb", "shutdown", "reboot", "init 0"}
		for _, d := range dangerous {
			if strings.Contains(normalized, d) {
				return nil, fmt.Errorf("comando bloqueado por segurança: contém '%s'", d)
			}
		}
	}

	// Montar comando Docker com flags de isolamento
	args := []string{
		"run", "--rm",
		"--network", "none",
		"--memory", "256m",
		"--cpus", "0.5",
		"-v", s.workDir + ":/workspace",
		"-w", "/workspace",
		image,
		shell, flag, code,
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = s.safeEnv()

	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &ExecResult{
		Output:   truncateOutput(string(output), 50000),
		Duration: duration,
		Language: language,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			// Se Docker falhar (ex: imagem não encontrada), tentar fallback direto
			return s.executeDirect(ctx, language, code, timeout)
		}
	}

	return result, nil
}

func (s *Service) executeBash(ctx context.Context, script string, timeout time.Duration) (*ExecResult, error) {
	if timeout <= 0 || timeout > s.maxTimeout {
		timeout = s.maxTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Security: bloquear comandos destrutivos
	normalized := strings.ToLower(script)
	dangerous := []string{"rm -rf /", "mkfs", "dd if=", ":(){", "fork bomb", "shutdown", "reboot", "init 0"}
	for _, d := range dangerous {
		if strings.Contains(normalized, d) {
			return nil, fmt.Errorf("comando bloqueado por segurança: contém '%s'", d)
		}
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Dir = s.workDir
	cmd.Env = s.safeEnv()

	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &ExecResult{
		Output:   truncateOutput(string(output), 50000),
		Duration: duration,
		Language: "bash",
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("execução falhou: %v", err)
		}
	}

	return result, nil
}

func (s *Service) executePython(ctx context.Context, script string, timeout time.Duration) (*ExecResult, error) {
	if timeout <= 0 || timeout > s.maxTimeout {
		timeout = s.maxTimeout
	}

	tmpFile := filepath.Join(s.workDir, fmt.Sprintf("eva_script_%d.py", time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		return nil, fmt.Errorf("erro ao criar script: %v", err)
	}
	defer os.Remove(tmpFile)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", tmpFile)
	cmd.Dir = s.workDir
	cmd.Env = s.safeEnv()

	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &ExecResult{
		Output:   truncateOutput(string(output), 50000),
		Duration: duration,
		Language: "python",
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("execução falhou: %v", err)
		}
	}

	return result, nil
}

func (s *Service) executeNode(ctx context.Context, script string, timeout time.Duration) (*ExecResult, error) {
	if timeout <= 0 || timeout > s.maxTimeout {
		timeout = s.maxTimeout
	}

	tmpFile := filepath.Join(s.workDir, fmt.Sprintf("eva_script_%d.js", time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		return nil, fmt.Errorf("erro ao criar script: %v", err)
	}
	defer os.Remove(tmpFile)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "node", tmpFile)
	cmd.Dir = s.workDir
	cmd.Env = s.safeEnv()

	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &ExecResult{
		Output:   truncateOutput(string(output), 50000),
		Duration: duration,
		Language: "node",
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("execução falhou: %v", err)
		}
	}

	return result, nil
}

// safeEnv retorna environment variables seguras (sem secrets)
func (s *Service) safeEnv() []string {
	safe := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + s.workDir,
		"LANG=en_US.UTF-8",
		"TERM=xterm",
	}
	return safe
}

func truncateOutput(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "\n... [output truncado]"
	}
	return s
}
