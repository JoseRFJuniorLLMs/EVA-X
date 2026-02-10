package lacan

import (
	"context"
	"database/sql"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"strings"
	"time"
)

// SignifierService rastreia significantes recorrentes (palavras-chave que se repetem) no Grafo
type SignifierService struct {
	client *graph.Neo4jClient
}

// NewSignifierService cria novo serviço com Neo4j
func NewSignifierService(client *graph.Neo4jClient) *SignifierService {
	return &SignifierService{client: client}
}

// Signifier representa um significante rastreado
type Signifier struct {
	Word            string
	Frequency       int
	Contexts        []string
	FirstOccurrence time.Time
	LastOccurrence  time.Time
	EmotionalCharge float64 // 0.0 (neutro) a 1.0 (altamente carregado)
}

// TrackSignifiers extrai e rastreia significantes emocionalmente carregados
func (s *SignifierService) TrackSignifiers(ctx context.Context, idosoID int64, text string) error {
	keywords := extractEmotionalKeywords(text)

	for _, word := range keywords {
		err := s.incrementSignifier(ctx, idosoID, word, text)
		if err != nil {
			return err
		}
	}

	return nil
}

// incrementSignifier incrementa frequência de um significante no Grafo
func (s *SignifierService) incrementSignifier(ctx context.Context, idosoID int64, word, contextStr string) error {
	// ✅ Proteção: Se Neo4j off, ignora
	if s.client == nil {
		return nil
	}

	query := `
		// Criar Person se não existir
		MERGE (p:Person {id: $idosoId})
		
		// Criar ou atualizar Significante
		MERGE (s:Significante {word: $word, idoso_id: $idosoId})
		ON CREATE SET 
			s.frequency = 1, 
			s.first_occurrence = datetime(), 
			s.last_occurrence = datetime(),
			s.contexts = [$context]
		ON MATCH SET 
			s.frequency = s.frequency + 1,
			s.last_occurrence = datetime(),
			s.contexts = s.contexts + $context
		
		// Criar evento de fala
		CREATE (e:Event {type: 'utterance', content: $context, timestamp: datetime()})
		MERGE (e)-[:EVOCA]->(s)
		MERGE (p)-[:EXPERIENCED]->(e)
	`

	params := map[string]interface{}{
		"idosoId": idosoID,
		"word":    word,
		"context": contextStr,
	}

	_, err := s.client.ExecuteWrite(ctx, query, params)
	return err
}

// GetKeySignifiers retorna os N significantes mais frequentes
func (s *SignifierService) GetKeySignifiers(ctx context.Context, idosoID int64, topN int) ([]Signifier, error) {
	// ✅ Proteção contra Panic: Se Neo4j estiver offline
	if s.client == nil {
		return []Signifier{}, nil
	}

	query := `
		MATCH (s:Significante {idoso_id: $idosoId})
		WHERE s.frequency >= 3
		RETURN s.word, s.frequency, s.contexts, s.first_occurrence, s.last_occurrence
		ORDER BY s.frequency DESC
		LIMIT $limit
	`

	records, err := s.client.ExecuteRead(ctx, query, map[string]interface{}{
		"idosoId": idosoID,
		"limit":   topN,
	})
	if err != nil {
		return nil, err
	}

	var signifiers []Signifier
	for _, record := range records {
		var sig Signifier

		word, _ := record.Get("s.word")
		freq, _ := record.Get("s.frequency")
		contexts, _ := record.Get("s.contexts")
		// first, _ := record.Get("s.first_occurrence")
		// last, _ := record.Get("s.last_occurrence")

		sig.Word = word.(string)
		sig.Frequency = int(freq.(int64))

		if ctxs, ok := contexts.([]interface{}); ok {
			for _, c := range ctxs {
				sig.Contexts = append(sig.Contexts, c.(string))
			}
		}

		sig.EmotionalCharge = calculateEmotionalCharge(sig.Word)
		signifiers = append(signifiers, sig)
	}

	return signifiers, nil
}

// ShouldInterpelSignifier decide se é momento de interpelar o significante
func (s *SignifierService) ShouldInterpelSignifier(ctx context.Context, idosoID int64, word string) (bool, error) {
	// Lógica similar, mas consultando grafo
	query := `
		MATCH (s:Significante {idoso_id: $idosoId, word: $word})
		RETURN s.frequency, s.last_interpellation
	`
	records, err := s.client.ExecuteRead(ctx, query, map[string]interface{}{"idosoId": idosoID, "word": word})
	if err != nil || len(records) == 0 {
		return false, err
	}

	freq, _ := records[0].Get("s.frequency")
	lastInterp, _ := records[0].Get("s.last_interpellation")

	frequency := int(freq.(int64))

	if frequency >= 5 {
		if lastInterp == nil {
			return true, nil
		}
		// Verificar data (simplificado por agora, neo4j date handling em Go pode ser chato)
		return true, nil
	}

	return false, nil
}

// MarkAsInterpelled marca que o significante foi interpelado
func (s *SignifierService) MarkAsInterpelled(ctx context.Context, idosoID int64, word string) error {
	query := `
		MATCH (s:Significante {idoso_id: $idosoId, word: $word})
		SET s.last_interpellation = datetime()
	`
	_, err := s.client.ExecuteWrite(ctx, query, map[string]interface{}{"idosoId": idosoID, "word": word})
	return err
}

// GenerateInterpellation gera frase para interpelar o significante
func GenerateInterpellation(word string, frequency int) string {
	return "Percebi que você frequentemente menciona a palavra '" + word + "'. " +
		"Ela apareceu " + string(rune(frequency)) + " vezes em nossas conversas. " +
		"O que essa palavra representa para você?"
}

// Helper functions (Mantidas)

func extractEmotionalKeywords(text string) []string {
	// Palavras com carga emocional (extração simples)
	emotionalWords := map[string]bool{
		"solidão":    true,
		"tristeza":   true,
		"medo":       true,
		"saudade":    true,
		"abandono":   true,
		"dor":        true,
		"sofrimento": true,
		"angústia":   true,
		"ansiedade":  true,
		"depressão":  true,
		"alegria":    true,
		"felicidade": true,
		"amor":       true,
		"morte":      true,
		"vida":       true,
		"família":    true,
		"filho":      true,
		"filha":      true,
		"esposa":     true,
		"marido":     true,
		"vazio":      true,
		"falta":      true,
		"perda":      true,
		"culpa":      true,
		"raiva":      true,
		"ódio":       true,
		"perdão":     true,
		"esperança":  true,
		"desespero":  true,
	}

	words := strings.Fields(strings.ToLower(text))
	var keywords []string

	for _, word := range words {
		// Remove pontuação
		cleaned := strings.Trim(word, ".,!?;:")
		if emotionalWords[cleaned] {
			keywords = append(keywords, cleaned)
		}
	}

	return keywords
}

func calculateEmotionalCharge(word string) float64 {
	// Palavras de alta carga emocional (Exemplo)
	highCharge := map[string]bool{
		"morte": true, "abandono": true, "solidão": true,
		"desespero": true, "ódio": true, "culpa": true,
		"vazio": true, "perda": true,
	}

	if highCharge[word] {
		return 1.0
	}

	return 0.5 // Carga média por padrão
}

// Keeping sql import just to mock the signature if needed but we removed it from struct
var _ = sql.ErrNoRows
