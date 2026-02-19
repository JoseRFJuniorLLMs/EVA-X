// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package selfcode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Service permite que EVA edite seu próprio código-fonte
// com restrições de segurança (somente branches eva/*)
type Service struct {
	projectDir string
}

// NewService cria o serviço de auto-programação
func NewService(projectDir string) *Service {
	return &Service{projectDir: projectDir}
}

// ReadSourceFile lê um arquivo do código-fonte
func (s *Service) ReadSourceFile(path string) (string, error) {
	fullPath := filepath.Join(s.projectDir, filepath.Clean(path))

	// Security: não sair do projeto
	absPath, _ := filepath.Abs(fullPath)
	absProject, _ := filepath.Abs(s.projectDir)
	if !strings.HasPrefix(absPath, absProject) {
		return "", fmt.Errorf("acesso negado: fora do diretório do projeto")
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("erro ao ler: %v", err)
	}

	// Limitar tamanho (500KB)
	if len(data) > 512*1024 {
		return "", fmt.Errorf("arquivo muito grande (max 500KB)")
	}

	return string(data), nil
}

// WriteSourceFile escreve um arquivo — SOMENTE em branches eva/*
func (s *Service) WriteSourceFile(path, content string) error {
	// Verificar que estamos em branch eva/*
	branch, err := s.getCurrentBranch()
	if err != nil {
		return fmt.Errorf("erro ao verificar branch: %v", err)
	}
	if !strings.HasPrefix(branch, "eva/") {
		return fmt.Errorf("escrita bloqueada: branch atual é '%s' — só posso escrever em branches eva/*", branch)
	}

	fullPath := filepath.Join(s.projectDir, filepath.Clean(path))

	// Security: não sair do projeto
	absPath, _ := filepath.Abs(fullPath)
	absProject, _ := filepath.Abs(s.projectDir)
	if !strings.HasPrefix(absPath, absProject) {
		return fmt.Errorf("acesso negado: fora do diretório do projeto")
	}

	// Criar diretórios pai se necessário
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório: %v", err)
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}

// CreateBranch cria uma nova branch eva/{name}
func (s *Service) CreateBranch(name string) error {
	branchName := "eva/" + strings.TrimPrefix(name, "eva/")
	return s.gitCommand("checkout", "-b", branchName)
}

// CommitChanges faz commit das mudanças atuais
func (s *Service) CommitChanges(message string) error {
	// Verificar branch
	branch, err := s.getCurrentBranch()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(branch, "eva/") {
		return fmt.Errorf("commit bloqueado: branch '%s' — só posso commitar em eva/*", branch)
	}

	if err := s.gitCommand("add", "-A"); err != nil {
		return fmt.Errorf("git add falhou: %v", err)
	}

	return s.gitCommand("commit", "-m", message)
}

// GetDiff mostra as mudanças não commitadas
func (s *Service) GetDiff() (string, error) {
	return s.gitOutput("diff")
}

// RunTests executa go test ./... com timeout de 2 minutos
func (s *Service) RunTests() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "./...")
	cmd.Dir = s.projectDir

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// ListBranches lista branches do projeto
func (s *Service) ListBranches() ([]string, error) {
	output, err := s.gitOutput("branch", "--list")
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, line := range strings.Split(output, "\n") {
		branch := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if branch != "" {
			branches = append(branches, branch)
		}
	}
	return branches, nil
}

// getCurrentBranch retorna o nome da branch atual
func (s *Service) getCurrentBranch() (string, error) {
	output, err := s.gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// gitCommand executa um comando git no diretório do projeto
func (s *Service) gitCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, string(output))
	}
	return nil
}

// gitOutput executa um comando git e retorna o output
func (s *Service) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%v: %s", err, string(output))
	}
	return string(output), nil
}
