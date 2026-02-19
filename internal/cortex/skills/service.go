// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CodeRunner função que executa código (do sandbox service)
type CodeRunner func(ctx context.Context, language, code string, timeout time.Duration) (output string, exitCode int, err error)

// Skill definição de uma skill
type Skill struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Language    string    `json:"language"` // bash, python, node
	Code        string    `json:"code"`
	Author      string    `json:"author"` // "eva" ou "human"
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	RunCount    int       `json:"run_count"`
}

// ExecResult resultado de execução de skill
type ExecResult struct {
	SkillName string `json:"skill_name"`
	Output    string `json:"output"`
	ExitCode  int    `json:"exit_code"`
	Duration  int64  `json:"duration_ms"`
}

// Service gerencia skills dinâmicas (runtime extensibility)
type Service struct {
	skillsDir string
	skills    map[string]*Skill
	runner    CodeRunner
	mu        sync.RWMutex
}

// NewService cria skills service
func NewService(skillsDir string) *Service {
	os.MkdirAll(skillsDir, 0755)
	svc := &Service{
		skillsDir: skillsDir,
		skills:    make(map[string]*Skill),
	}
	svc.loadFromDisk()
	return svc
}

// SetRunner configura o executor de código
func (s *Service) SetRunner(runner CodeRunner) {
	s.runner = runner
}

// Create cria nova skill
func (s *Service) Create(name, description, language, code, author string) (*Skill, error) {
	name = sanitizeName(name)
	if name == "" {
		return nil, fmt.Errorf("nome da skill é obrigatório")
	}

	language = strings.ToLower(language)
	validLangs := map[string]bool{"bash": true, "python": true, "node": true, "sh": true, "py": true, "js": true}
	if !validLangs[language] {
		return nil, fmt.Errorf("linguagem '%s' não suportada (use bash, python ou node)", language)
	}

	// Normalizar
	switch language {
	case "sh":
		language = "bash"
	case "py":
		language = "python"
	case "js":
		language = "node"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	version := 1
	if existing, ok := s.skills[name]; ok {
		version = existing.Version + 1
	}

	skill := &Skill{
		Name:        name,
		Description: description,
		Language:    language,
		Code:        code,
		Author:      author,
		Version:     version,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	s.skills[name] = skill

	// Salvar em disco
	if err := s.saveToDisk(skill); err != nil {
		log.Printf("⚠️ [SKILLS] Erro ao salvar skill %s: %v", name, err)
	}

	log.Printf("🧩 [SKILLS] Skill criada: %s v%d (%s) by %s", name, version, language, author)
	return skill, nil
}

// List lista todas as skills
func (s *Service) List() []*Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Skill
	for _, skill := range s.skills {
		result = append(result, skill)
	}
	return result
}

// Get obtém uma skill por nome
func (s *Service) Get(name string) (*Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	skill, ok := s.skills[sanitizeName(name)]
	if !ok {
		return nil, fmt.Errorf("skill '%s' não encontrada", name)
	}
	return skill, nil
}

// Execute executa uma skill
func (s *Service) Execute(ctx context.Context, name string, args map[string]interface{}) (*ExecResult, error) {
	s.mu.RLock()
	skill, ok := s.skills[sanitizeName(name)]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("skill '%s' não encontrada", name)
	}
	s.mu.RUnlock()

	if s.runner == nil {
		return nil, fmt.Errorf("code runner não configurado")
	}

	// Injetar argumentos como variáveis de ambiente no código
	code := skill.Code
	for k, v := range args {
		code = strings.ReplaceAll(code, fmt.Sprintf("${%s}", k), fmt.Sprintf("%v", v))
		code = strings.ReplaceAll(code, fmt.Sprintf("$%s", k), fmt.Sprintf("%v", v))
	}

	start := time.Now()
	output, exitCode, err := s.runner(ctx, skill.Language, code, 2*time.Minute)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("execução falhou: %v", err)
	}

	// Atualizar stats
	s.mu.Lock()
	skill.RunCount++
	s.mu.Unlock()

	return &ExecResult{
		SkillName: name,
		Output:    output,
		ExitCode:  exitCode,
		Duration:  duration.Milliseconds(),
	}, nil
}

// Delete remove uma skill
func (s *Service) Delete(name string) error {
	name = sanitizeName(name)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.skills[name]; !ok {
		return fmt.Errorf("skill '%s' não encontrada", name)
	}

	delete(s.skills, name)

	// Remover arquivo
	filePath := filepath.Join(s.skillsDir, name+".json")
	os.Remove(filePath)

	log.Printf("🧩 [SKILLS] Skill removida: %s", name)
	return nil
}

// saveToDisk salva skill como JSON
func (s *Service) saveToDisk(skill *Skill) error {
	data, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return err
	}
	filePath := filepath.Join(s.skillsDir, skill.Name+".json")
	return os.WriteFile(filePath, data, 0644)
}

// loadFromDisk carrega skills do diretório
func (s *Service) loadFromDisk() {
	entries, err := os.ReadDir(s.skillsDir)
	if err != nil {
		return
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.skillsDir, entry.Name()))
		if err != nil {
			continue
		}

		var skill Skill
		if err := json.Unmarshal(data, &skill); err != nil {
			continue
		}

		s.skills[skill.Name] = &skill
		count++
	}

	if count > 0 {
		log.Printf("🧩 [SKILLS] %d skills carregadas do disco", count)
	}
}

// sanitizeName normaliza nome de skill
func sanitizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}
