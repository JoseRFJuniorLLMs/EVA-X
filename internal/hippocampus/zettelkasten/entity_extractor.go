package zettelkasten

import (
	"regexp"
	"strings"
)

// EntityExtractor extrai entidades de texto (pessoas, lugares, datas)
type EntityExtractor struct {
	personPatterns []*regexp.Regexp
	placePatterns  []*regexp.Regexp
	datePatterns   []*regexp.Regexp
	eventPatterns  []*regexp.Regexp
}

// NewEntityExtractor cria novo extrator
func NewEntityExtractor() *EntityExtractor {
	ee := &EntityExtractor{}
	ee.compilePatterns()
	return ee
}

func (ee *EntityExtractor) compilePatterns() {
	// Padrões para pessoas (relações familiares e nomes)
	personPatterns := []string{
		// Relações familiares
		`(?i)(?:meu|minha)\s+(pai|mãe|avô|avó|filho|filha|neto|neta|esposo|esposa|marido|mulher|irmão|irmã|tio|tia|primo|prima|sobrinho|sobrinha|cunhado|cunhada|sogro|sogra|genro|nora|padrinho|madrinha|afilhado|afilhada)(?:\s+([A-ZÁÉÍÓÚÂÊÔÃÕÇ][a-záéíóúâêôãõç]+))?`,
		// Nomes próprios após "o/a"
		`(?i)(?:o|a)\s+([A-ZÁÉÍÓÚÂÊÔÃÕÇ][a-záéíóúâêôãõç]+)(?:\s+(?:disse|falou|era|foi|estava|é|trabalha|mora))`,
		// Dona/Seu + nome
		`(?i)(?:dona?|seu)\s+([A-ZÁÉÍÓÚÂÊÔÃÕÇ][a-záéíóúâêôãõç]+)`,
		// Dr./Dra. + nome
		`(?i)(?:dr\.?|dra\.?|doutor|doutora)\s+([A-ZÁÉÍÓÚÂÊÔÃÕÇ][a-záéíóúâêôãõç]+)`,
	}

	// Padrões para lugares
	placePatterns := []string{
		// Cidades/Estados
		`(?i)(?:em|de|para|na|no)\s+([A-ZÁÉÍÓÚÂÊÔÃÕÇ][a-záéíóúâêôãõç]+(?:\s+(?:do|da|de|dos|das)\s+[A-ZÁÉÍÓÚÂÊÔÃÕÇ][a-záéíóúâêôãõç]+)?)`,
		// Lugares específicos
		`(?i)(?:na|no|a)\s+(fazenda|sítio|chácara|casa|apartamento|igreja|escola|hospital|praça|rua|avenida)\s+(?:de\s+)?([A-ZÁÉÍÓÚÂÊÔÃÕÇ][a-záéíóúâêôãõç\s]+)`,
		// Bairros
		`(?i)(?:bairro|vila)\s+(?:de\s+)?([A-ZÁÉÍÓÚÂÊÔÃÕÇ][a-záéíóúâêôãõç\s]+)`,
	}

	// Padrões para datas e épocas
	datePatterns := []string{
		// Anos
		`(?i)(?:em|no|ano\s+de)\s+(19[0-9]{2}|20[0-2][0-9])`,
		// Décadas
		`(?i)(?:nos\s+)?anos\s+((?:19)?[2-9]0|(?:20)?[0-2]0)`,
		// Épocas da vida
		`(?i)quando\s+(?:eu\s+)?(?:era|tinha|fazia|trabalhava)\s+(\d+)\s+anos?`,
		// Períodos
		`(?i)(?:na|durante\s+a)\s+(guerra|ditadura|infância|juventude|adolescência)`,
	}

	// Padrões para eventos
	eventPatterns := []string{
		// Casamento, nascimento, morte
		`(?i)(?:quando|no\s+dia\s+(?:que|do))\s+(?:eu\s+)?(?:casei|me\s+casei|nasceu|morreu|faleceu|formei|me\s+formei|aposentei)`,
		// Festas/comemorações
		`(?i)(?:no|na)\s+(natal|páscoa|ano\s+novo|aniversário|casamento|batizado|formatura)`,
	}

	ee.personPatterns = compilePatternList(personPatterns)
	ee.placePatterns = compilePatternList(placePatterns)
	ee.datePatterns = compilePatternList(datePatterns)
	ee.eventPatterns = compilePatternList(eventPatterns)
}

func compilePatternList(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// Extract extrai todas as entidades do texto
func (ee *EntityExtractor) Extract(text string) []Entity {
	var entities []Entity
	seen := make(map[string]bool)

	// Extrair pessoas
	for _, pattern := range ee.personPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				role := ""
				value := ""

				// Verificar se é relação familiar + nome
				if len(match) >= 3 && match[2] != "" {
					role = strings.ToLower(match[1])
					value = strings.TrimSpace(match[2])
				} else {
					value = strings.TrimSpace(match[1])
				}

				// Ignorar se muito curto ou já visto
				if len(value) < 2 || seen[strings.ToLower(value)] {
					continue
				}

				// Verificar se é nome próprio (começa com maiúscula)
				if isProperName(value) || role != "" {
					seen[strings.ToLower(value)] = true
					entities = append(entities, Entity{
						Type:  "person",
						Value: value,
						Role:  role,
					})
				}
			}
		}
	}

	// Extrair lugares
	for _, pattern := range ee.placePatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			for i := 1; i < len(match); i++ {
				value := strings.TrimSpace(match[i])
				if len(value) > 2 && !seen[strings.ToLower(value)] && isProperName(value) {
					// Filtrar palavras comuns que não são lugares
					if !isCommonWord(value) {
						seen[strings.ToLower(value)] = true
						entities = append(entities, Entity{
							Type:  "place",
							Value: value,
						})
					}
				}
			}
		}
	}

	// Extrair datas/épocas
	for _, pattern := range ee.datePatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				value := strings.TrimSpace(match[1])
				key := "date:" + strings.ToLower(value)
				if !seen[key] {
					seen[key] = true
					entities = append(entities, Entity{
						Type:  "date",
						Value: value,
					})
				}
			}
		}
	}

	// Extrair eventos
	for _, pattern := range ee.eventPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) >= 1 {
				value := strings.TrimSpace(match[0])
				key := "event:" + strings.ToLower(value)
				if !seen[key] && len(value) > 5 {
					seen[key] = true
					entities = append(entities, Entity{
						Type:  "event",
						Value: ee.cleanEventText(value),
					})
				}
			}
		}
	}

	return entities
}

// ExtractPeople extrai apenas pessoas
func (ee *EntityExtractor) ExtractPeople(text string) []Entity {
	all := ee.Extract(text)
	var people []Entity
	for _, e := range all {
		if e.Type == "person" {
			people = append(people, e)
		}
	}
	return people
}

// ExtractPlaces extrai apenas lugares
func (ee *EntityExtractor) ExtractPlaces(text string) []Entity {
	all := ee.Extract(text)
	var places []Entity
	for _, e := range all {
		if e.Type == "place" {
			places = append(places, e)
		}
	}
	return places
}

// Helper: verifica se é nome próprio
func isProperName(s string) bool {
	if len(s) == 0 {
		return false
	}
	firstChar := s[0]
	return firstChar >= 'A' && firstChar <= 'Z'
}

// Helper: verifica se é palavra comum (não é entidade)
func isCommonWord(s string) bool {
	commonWords := map[string]bool{
		"Brasil": false, "São Paulo": false, "Rio": false, // Lugares válidos
		"Casa": true, "Dia": true, "Ano": true, "Mês": true,
		"Tempo": true, "Vez": true, "Coisa": true, "Gente": true,
		"Pessoa": true, "Vida": true, "Mundo": true, "Hora": true,
		"Lugar": true, "Parte": true, "Lado": true, "Fim": true,
		"Tipo": true, "Jeito": true, "Modo": true, "Forma": true,
	}

	if val, exists := commonWords[s]; exists {
		return val
	}
	return false
}

// Helper: limpa texto de evento
func (ee *EntityExtractor) cleanEventText(text string) string {
	// Remover "quando eu" do início
	text = regexp.MustCompile(`(?i)^quando\s+(?:eu\s+)?`).ReplaceAllString(text, "")
	// Remover "no dia que" do início
	text = regexp.MustCompile(`(?i)^no\s+dia\s+(?:que|do)\s+`).ReplaceAllString(text, "")
	// Capitalizar primeira letra
	if len(text) > 0 {
		text = strings.ToUpper(text[:1]) + text[1:]
	}
	return text
}

// ============================================================================
// ADVANCED EXTRACTION (usando heurísticas mais sofisticadas)
// ============================================================================

// ExtractWithContext extrai entidades considerando contexto
func (ee *EntityExtractor) ExtractWithContext(text string, previousEntities []Entity) []Entity {
	entities := ee.Extract(text)

	// Se menciona pronomes (ele, ela) e temos contexto anterior, resolver referência
	pronounPatterns := map[string]string{
		`(?i)\bele\b`:  "person",
		`(?i)\bela\b`:  "person",
		`(?i)\beles\b`: "person",
		`(?i)\belas\b`: "person",
		`(?i)\blá\b`:   "place",
	}

	for pronoun, entityType := range pronounPatterns {
		re := regexp.MustCompile(pronoun)
		if re.MatchString(text) {
			// Encontrar última entidade do tipo correto
			for i := len(previousEntities) - 1; i >= 0; i-- {
				if previousEntities[i].Type == entityType {
					// Adicionar como referência
					entities = append(entities, Entity{
						Type:  entityType,
						Value: previousEntities[i].Value,
						Role:  "referência",
					})
					break
				}
			}
		}
	}

	return entities
}

// ExtractRelationships extrai relacionamentos entre entidades
func (ee *EntityExtractor) ExtractRelationships(text string, entities []Entity) []EntityRelationship {
	var relationships []EntityRelationship

	// Padrões de relacionamento
	relPatterns := []struct {
		pattern  string
		relType  string
	}{
		{`(?i)(\w+)\s+(?:é|era|foi)\s+(?:meu|minha)\s+(pai|mãe|avô|avó|filho|filha)`, "family"},
		{`(?i)(\w+)\s+(?:casou|se\s+casou)\s+com\s+(\w+)`, "married"},
		{`(?i)(\w+)\s+(?:mora|morava|trabalha|trabalhava)\s+(?:em|no|na)\s+(\w+)`, "location"},
		{`(?i)(\w+)\s+(?:e|com)\s+(\w+)\s+(?:eram|são|foram)\s+amigos`, "friends"},
	}

	for _, rp := range relPatterns {
		re := regexp.MustCompile(rp.pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				rel := EntityRelationship{
					Entity1:      match[1],
					Entity2:      match[2],
					Relationship: rp.relType,
				}
				relationships = append(relationships, rel)
			}
		}
	}

	return relationships
}

// EntityRelationship representa um relacionamento entre entidades
type EntityRelationship struct {
	Entity1      string `json:"entity1"`
	Entity2      string `json:"entity2"`
	Relationship string `json:"relationship"`
}
