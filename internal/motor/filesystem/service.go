// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileInfo informações de um arquivo/diretório
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
}

// Service provê acesso sandboxed ao filesystem
type Service struct {
	baseDir string
}

// NewService cria um serviço de filesystem limitado ao baseDir
func NewService(baseDir string) *Service {
	return &Service{baseDir: baseDir}
}

// safePath garante que o path está dentro do sandbox
func (s *Service) safePath(path string) (string, error) {
	// Resolver path absoluto
	resolved := filepath.Join(s.baseDir, filepath.Clean(path))
	resolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("caminho inválido: %v", err)
	}

	// Verificar que está dentro do baseDir
	baseAbs, _ := filepath.Abs(s.baseDir)
	if !strings.HasPrefix(resolved, baseAbs) {
		return "", fmt.Errorf("acesso negado: caminho fora do diretório permitido")
	}

	return resolved, nil
}

// ReadFile lê o conteúdo de um arquivo (max 1MB)
func (s *Service) ReadFile(path string) (string, error) {
	safePath, err := s.safePath(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(safePath)
	if err != nil {
		return "", fmt.Errorf("arquivo não encontrado: %v", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("'%s' é um diretório, não um arquivo", path)
	}
	if info.Size() > 1024*1024 {
		return "", fmt.Errorf("arquivo muito grande (%.1f MB, máximo 1 MB)", float64(info.Size())/1024/1024)
	}

	data, err := os.ReadFile(safePath)
	if err != nil {
		return "", fmt.Errorf("erro ao ler: %v", err)
	}

	return string(data), nil
}

// WriteFile escreve conteúdo em um arquivo
func (s *Service) WriteFile(path, content string) error {
	safePath, err := s.safePath(path)
	if err != nil {
		return err
	}

	// Criar diretórios pai se necessário
	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório: %v", err)
	}

	return os.WriteFile(safePath, []byte(content), 0644)
}

// ListDirectory lista o conteúdo de um diretório
func (s *Service) ListDirectory(path string) ([]FileInfo, error) {
	safePath, err := s.safePath(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(safePath)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar diretório: %v", err)
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			Size:    info.Size(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	return files, nil
}

// SearchFiles busca arquivos por padrão glob
func (s *Service) SearchFiles(pattern string) ([]string, error) {
	baseAbs, _ := filepath.Abs(s.baseDir)

	var results []string
	err := filepath.Walk(baseAbs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}

		// Match pelo nome do arquivo
		matched, _ := filepath.Match(pattern, info.Name())
		if matched || strings.Contains(strings.ToLower(info.Name()), strings.ToLower(pattern)) {
			relPath, _ := filepath.Rel(baseAbs, path)
			results = append(results, relPath)

			// Limitar resultados
			if len(results) >= 50 {
				return filepath.SkipAll
			}
		}
		return nil
	})

	return results, err
}
