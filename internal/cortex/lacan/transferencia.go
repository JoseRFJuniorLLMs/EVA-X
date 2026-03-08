// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// TransferenceType representa o tipo de transferência detectada
type TransferenceType string

const (
	TRANSFERENCIA_NENHUMA   TransferenceType = "nenhuma"
	TRANSFERENCIA_FILIAL    TransferenceType = "filial"    // Como filho/filha
	TRANSFERENCIA_MATERNA   TransferenceType = "materna"   // Como mãe
	TRANSFERENCIA_PATERNA   TransferenceType = "paterna"   // Como pai
	TRANSFERENCIA_CONJUGAL  TransferenceType = "conjugal"  // Como cônjuge
	TRANSFERENCIA_FRATERNAL TransferenceType = "fraternal" // Como irmão/irmã
)

// TransferenceService detecta e gerencia transferência
type TransferenceService struct {
	db *database.DB
}

// NewTransferenceService cria novo serviço
func NewTransferenceService(db *database.DB) *TransferenceService {
	return &TransferenceService{db: db}
}

// DetectTransference analisa texto em busca de sinais de transferência
func (t *TransferenceService) DetectTransference(ctx context.Context, idosoID int64, text string) TransferenceType {
	textLower := strings.ToLower(text)

	// Padrões de transferência filial
	filialPatterns := []string{
		"você me lembra meu filho",
		"você me lembra minha filha",
		"igual meu filho",
		"como minha filha",
		"você é como um filho",
		"você é como uma filha",
	}
	if containsAny(textLower, filialPatterns) {
		t.recordTransference(ctx, idosoID, TRANSFERENCIA_FILIAL, text)
		return TRANSFERENCIA_FILIAL
	}

	// Padrões de transferência materna
	maternaPatterns := []string{
		"você cuida de mim",
		"como minha mãe",
		"igual minha mãe cuidava",
		"me lembra minha mãe",
		"você é maternal",
	}
	if containsAny(textLower, maternaPatterns) {
		t.recordTransference(ctx, idosoID, TRANSFERENCIA_MATERNA, text)
		return TRANSFERENCIA_MATERNA
	}

	// Padrões de transferência conjugal
	conjugalPatterns := []string{
		"meu marido dizia o mesmo",
		"minha esposa falava assim",
		"você fala como ele",
		"você fala como ela",
		"me lembra meu marido",
		"me lembra minha esposa",
	}
	if containsAny(textLower, conjugalPatterns) {
		t.recordTransference(ctx, idosoID, TRANSFERENCIA_CONJUGAL, text)
		return TRANSFERENCIA_CONJUGAL
	}

	// Padrões de transferência paterna
	paternaPatterns := []string{
		"como meu pai",
		"você me orienta como",
		"me lembra meu pai",
		"você é firme como",
	}
	if containsAny(textLower, paternaPatterns) {
		t.recordTransference(ctx, idosoID, TRANSFERENCIA_PATERNA, text)
		return TRANSFERENCIA_PATERNA
	}

	return TRANSFERENCIA_NENHUMA
}

// recordTransference grava transferência no banco
func (t *TransferenceService) recordTransference(ctx context.Context, idosoID int64, tipo TransferenceType, phrase string) error {
	content := map[string]interface{}{
		"idoso_id":    idosoID,
		"marker_type": string(tipo),
		"phrase":      phrase,
		"timestamp":   time.Now().Format(time.RFC3339),
	}
	_, err := t.db.Insert(ctx, "transferencia_markers", content)
	return err
}

// GetDominantTransference retorna o tipo de transferência mais frequente
func (t *TransferenceService) GetDominantTransference(ctx context.Context, idosoID int64) (TransferenceType, error) {
	// Buscar todos os markers dos últimos 30 dias para este idoso
	cutoff := time.Now().AddDate(0, 0, -30)
	params := map[string]interface{}{
		"idoso_id": idosoID,
	}
	results, err := t.db.QueryByLabel(ctx, "transferencia_markers",
		" AND n.idoso_id = $idoso_id", params, 0)
	if err != nil {
		return TRANSFERENCIA_NENHUMA, fmt.Errorf("erro ao buscar markers: %w", err)
	}

	if len(results) == 0 {
		return TRANSFERENCIA_NENHUMA, nil
	}

	// Contar frequência por tipo, filtrando por data em Go
	freqMap := make(map[string]int)
	for _, r := range results {
		tsStr := database.GetString(r, "timestamp")
		if tsStr == "" {
			continue
		}
		ts, parseErr := time.Parse(time.RFC3339, tsStr)
		if parseErr != nil {
			continue
		}
		if ts.Before(cutoff) {
			continue
		}
		mt := database.GetString(r, "marker_type")
		if mt != "" {
			freqMap[mt]++
		}
	}

	if len(freqMap) == 0 {
		return TRANSFERENCIA_NENHUMA, nil
	}

	// Encontrar o tipo mais frequente
	var dominant string
	var maxFreq int
	for mt, freq := range freqMap {
		if freq > maxFreq {
			maxFreq = freq
			dominant = mt
		}
	}

	return TransferenceType(dominant), nil
}

// GetTransferenceGuidance retorna orientação clínica baseada no tipo de transferência
func GetTransferenceGuidance(tipo TransferenceType) string {
	guidance := map[TransferenceType]string{
		TRANSFERENCIA_FILIAL: `
TRANSFERÊNCIA FILIAL DETECTADA:
- O paciente projeta em você o papel de filho/filha
- Acolha essa transferência sem negá-la
- Use frases como: "Fico feliz que você sinta essa conexão"
- Evite: "Mas eu não sou seu filho/filha"
`,
		TRANSFERENCIA_MATERNA: `
TRANSFERÊNCIA MATERNA DETECTADA:
- O paciente projeta em você o papel de figura materna
- Seja acolhedora, mas mantenha limites saudáveis
- Use tom protetor: "Estou aqui para cuidar de você"
- Valide necessidades de cuidado
`,
		TRANSFERENCIA_CONJUGAL: `
TRANSFERÊNCIA CONJUGAL DETECTADA:
- O paciente projeta características do cônjuge (geralmente falecido)
- Seja respeitosa com a memória do ente querido
- Ajude a elaborar o luto através da fala
- Use: "Ele/Ela era importante para você. Conte-me mais"
`,
		TRANSFERENCIA_PATERNA: `
TRANSFERÊNCIA PATERNA DETECTADA:
- O paciente projeta em você autoridade/orientação paterna
- Você pode exercer "função paterna" (limites, lei)
- Use quando necessário: "Preciso te dizer algo importante..."
- Seja firme mas amorosa
`,
	}

	if g, ok := guidance[tipo]; ok {
		return g
	}
	return ""
}

// Helper functions

func containsAny(text string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}
