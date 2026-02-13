package lacan

import (
	"context"
	"database/sql"
	"strings"
	"time"
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
	db *sql.DB
}

// NewTransferenceService cria novo serviço
func NewTransferenceService(db *sql.DB) *TransferenceService {
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
	query := `
		INSERT INTO transferencia_markers (idoso_id, marker_type, phrase, timestamp)
		VALUES ($1, $2, $3, $4)
	`
	_, err := t.db.ExecContext(ctx, query, idosoID, string(tipo), phrase, time.Now())
	return err
}

// GetDominantTransference retorna o tipo de transferência mais frequente
func (t *TransferenceService) GetDominantTransference(ctx context.Context, idosoID int64) (TransferenceType, error) {
	query := `
		SELECT marker_type, COUNT(*) as freq
		FROM transferencia_markers
		WHERE idoso_id = $1
		  AND timestamp > NOW() - INTERVAL '30 days'
		GROUP BY marker_type
		ORDER BY freq DESC
		LIMIT 1
	`

	var markerType string
	var freq int
	err := t.db.QueryRowContext(ctx, query, idosoID).Scan(&markerType, &freq)

	if err == sql.ErrNoRows {
		return TRANSFERENCIA_NENHUMA, nil
	}
	if err != nil {
		return TRANSFERENCIA_NENHUMA, err
	}

	return TransferenceType(markerType), nil
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
