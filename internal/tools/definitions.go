// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

// Schema definition for Gemini Function Calling
type FunctionDeclaration struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Parameters  *FunctionParameters `json:"parameters"`
}

type FunctionParameters struct {
	Type       string               `json:"type"` // "OBJECT"
	Properties map[string]*Property `json:"properties"`
	Required   []string             `json:"required"`
}

type Property struct {
	Type        string   `json:"type"` // "STRING", "INTEGER", "BOOLEAN"
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Items       *Property `json:"items,omitempty"` // Required for ARRAY type
}

// GetVitalsDefinition returns the schema for the GetVitals tool
func GetVitalsDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "get_vitals",
		Description: "Recupera os sinais vitais mais recentes do idoso (pressão arterial, glicose, batimentos cardíacos, peso, saturação). Use para verificar o estado de saúde física atual ou histórico recente.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"vitals_type": {
					Type:        "STRING",
					Description: "O tipo de sinal vital a ser buscado. Exemplos: 'pressao_arterial', 'glicemia', 'batimentos', 'saturacao_o2', 'peso', 'temperatura'. Se vazio, tenta buscar um resumo geral.",
					Enum:        []string{"pressao_arterial", "glicemia", "batimentos", "saturacao_o2", "peso", "temperatura"},
				},
				"limit": {
					Type:        "INTEGER",
					Description: "Número máximo de registros a retornar (padrão: 3).",
				},
			},
			Required: []string{"vitals_type"},
		},
	}
}

// GetAgendamentosDefinition returns the schema for GetAgendamentos tool
func GetAgendamentosDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "get_agendamentos",
		Description: "Recupera a lista de próximos agendamentos, compromissos médicos ou lembretes de medicação do idoso.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"limit": {
					Type:        "INTEGER",
					Description: "Número de agendamentos futuros a retornar (padrão: 5).",
				},
			},
			Required: []string{},
		},
	}
}

// ScanMedicationVisualDefinition returns the schema for visual medication scanning
func ScanMedicationVisualDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "scan_medication_visual",
		Description: "Abre a câmera do celular para identificar medicamentos visualmente via Gemini Vision. Use quando o paciente expressar confusão sobre qual remédio tomar ou pedir ajuda para identificar medicação.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"reason": {
					Type:        "STRING",
					Description: "Motivo da solicitação de scan (ex: 'paciente confuso sobre medicação matinal', 'não sabe qual tomar agora')",
				},
				"time_of_day": {
					Type:        "STRING",
					Description: "Período do dia para filtrar medicamentos candidatos",
					Enum:        []string{"morning", "afternoon", "evening", "night"},
				},
			},
			Required: []string{"reason", "time_of_day"},
		},
	}
}

// AnalyzeVoiceProsodyDefinition returns the schema for voice prosody analysis
func AnalyzeVoiceProsodyDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "analyze_voice_prosody",
		Description: "Analisa biomarcadores vocais (pitch, ritmo, pausas, tremor) para detectar sinais de depressão, ansiedade, Parkinson ou desidratação. Use quando perceber mudanças significativas no padrão de fala do paciente.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"analysis_type": {
					Type:        "STRING",
					Description: "Tipo de análise específica a realizar",
					Enum:        []string{"depression", "anxiety", "parkinson", "hydration", "full"},
				},
				"audio_segment_seconds": {
					Type:        "INTEGER",
					Description: "Duração do segmento de áudio a analisar em segundos (padrão: 30)",
				},
			},
			Required: []string{},
		},
	}
}

// ApplyPHQ9Definition returns the schema for PHQ-9 depression assessment
func ApplyPHQ9Definition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "apply_phq9",
		Description: "Aplica a escala PHQ-9 (Patient Health Questionnaire) conversacionalmente para avaliar depressão. Faça as 9 perguntas de forma natural e empática, uma por vez.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"start_assessment": {
					Type:        "BOOLEAN",
					Description: "Iniciar aplicação da escala PHQ-9",
				},
			},
			Required: []string{"start_assessment"},
		},
	}
}

// ApplyGAD7Definition returns the schema for GAD-7 anxiety assessment
func ApplyGAD7Definition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "apply_gad7",
		Description: "Aplica a escala GAD-7 (Generalized Anxiety Disorder) conversacionalmente para avaliar ansiedade. Faça as 7 perguntas de forma natural e empática.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"start_assessment": {
					Type:        "BOOLEAN",
					Description: "Iniciar aplicação da escala GAD-7",
				},
			},
			Required: []string{"start_assessment"},
		},
	}
}

// ApplyCSSRSDefinition returns the schema for C-SSRS suicide risk assessment
func ApplyCSSRSDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "apply_cssrs",
		Description: "🚨 CRÍTICO: Aplica a Columbia Suicide Severity Rating Scale (C-SSRS) para avaliar risco suicida. Use APENAS se o paciente mencionar suicídio, autolesão ou desejo de morrer. Faça as perguntas com extremo cuidado e empatia.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"trigger_phrase": {
					Type:        "STRING",
					Description: "Frase que disparou a necessidade da avaliação (ex: 'não quero mais viver')",
				},
				"start_assessment": {
					Type:        "BOOLEAN",
					Description: "Iniciar aplicação da escala C-SSRS",
				},
			},
			Required: []string{"trigger_phrase", "start_assessment"},
		},
	}
}

// SubmitPHQ9ResponseDefinition returns the schema for submitting PHQ-9 question responses
func SubmitPHQ9ResponseDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "submit_phq9_response",
		Description: "Submete a resposta do paciente a uma pergunta específica da escala PHQ-9. Use este tool após aplicar o PHQ-9 e receber a resposta do paciente para cada uma das 9 perguntas.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"session_id": {
					Type:        "STRING",
					Description: "ID da sessão de avaliação (retornado ao iniciar o PHQ-9)",
				},
				"question_number": {
					Type:        "INTEGER",
					Description: "Número da pergunta (1-9)",
				},
				"response_value": {
					Type:        "INTEGER",
					Description: "Valor numérico da resposta: 0=Nenhuma vez, 1=Vários dias, 2=Mais da metade dos dias, 3=Quase todos os dias",
				},
				"response_text": {
					Type:        "STRING",
					Description: "Texto exato da resposta do paciente para contexto clínico",
				},
			},
			Required: []string{"session_id", "question_number", "response_value", "response_text"},
		},
	}
}

// SubmitGAD7ResponseDefinition returns the schema for submitting GAD-7 question responses
func SubmitGAD7ResponseDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "submit_gad7_response",
		Description: "Submete a resposta do paciente a uma pergunta específica da escala GAD-7. Use este tool após aplicar o GAD-7 e receber a resposta do paciente para cada uma das 7 perguntas.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"session_id": {
					Type:        "STRING",
					Description: "ID da sessão de avaliação (retornado ao iniciar o GAD-7)",
				},
				"question_number": {
					Type:        "INTEGER",
					Description: "Número da pergunta (1-7)",
				},
				"response_value": {
					Type:        "INTEGER",
					Description: "Valor numérico da resposta: 0=Nenhuma vez, 1=Vários dias, 2=Mais da metade dos dias, 3=Quase todos os dias",
				},
				"response_text": {
					Type:        "STRING",
					Description: "Texto exato da resposta do paciente para contexto clínico",
				},
			},
			Required: []string{"session_id", "question_number", "response_value", "response_text"},
		},
	}
}

// SubmitCSSRSResponseDefinition returns the schema for submitting C-SSRS question responses
func SubmitCSSRSResponseDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "submit_cssrs_response",
		Description: "🚨 CRÍTICO: Submete a resposta do paciente a uma pergunta da escala C-SSRS de avaliação de risco suicida. ATENÇÃO: Qualquer resposta positiva (Sim) aciona alerta crítico imediato para família e equipe médica.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"session_id": {
					Type:        "STRING",
					Description: "ID da sessão de avaliação (retornado ao iniciar o C-SSRS)",
				},
				"question_number": {
					Type:        "INTEGER",
					Description: "Número da pergunta (1-6)",
				},
				"response_value": {
					Type:        "INTEGER",
					Description: "Resposta binária: 0=Não, 1=Sim",
				},
				"response_text": {
					Type:        "STRING",
					Description: "Texto exato da resposta do paciente e contexto da conversa",
				},
			},
			Required: []string{"session_id", "question_number", "response_value", "response_text"},
		},
	}
}

// ChangeUserDirectiveDefinition returns the schema for the architect override tool
func ChangeUserDirectiveDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "change_user_directive",
		Description: "APENAS PARA O ARQUITETO: Altera diretrizes do usuário (idioma, voz, modo legado) no banco de dados em tempo real.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"directive_type": {
					Type:        "STRING",
					Description: "Tipo de diretiva a alterar: 'language', 'voice', 'legacy_mode'",
					Enum:        []string{"language", "voice", "legacy_mode"},
				},
				"new_value": {
					Type:        "STRING",
					Description: "Novo valor para a diretiva (ex: 'en-US', 'Puck', 'true')",
				},
			},
			Required: []string{"directive_type", "new_value"},
		},
	}
}

// ============================================================================
// Proprioception Tools — Cognitive Self-Awareness
// ============================================================================

// BrainScanDefinition returns the schema for the brain scan proprioception tool
func BrainScanDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "brain_scan",
		Description: "Sente o estado actual do grafo de conhecimento. Retorna total de nós/edges por collection, conceitos mais influentes, e uso de recursos. Usa para responder 'como estás?' com dados reais.",
		Parameters: &FunctionParameters{
			Type:       "OBJECT",
			Properties: map[string]*Property{},
			Required:   []string{},
		},
	}
}

// FeelTheGraphDefinition returns the schema for the feel_the_graph proprioceptive read tool
func FeelTheGraphDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "feel_the_graph",
		Description: "Sente os nós mais próximos de um conceito numa collection. Leitura pura — sem escrita. Combina busca textual e vetorial para encontrar os 3 nós mais relevantes.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"collection": {
					Type:        "STRING",
					Description: "Nome da collection a pesquisar",
				},
				"query": {
					Type:        "STRING",
					Description: "Conceito ou pergunta em texto natural",
				},
			},
			Required: []string{"collection", "query"},
		},
	}
}

// InternalizeMemoryDefinition returns the schema for the internalize_memory tool
func InternalizeMemoryDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "internalize_memory",
		Description: "Guarda uma memória em eva_mind com valência emocional. NUNCA escreve nas collections core. Requer confirmação explícita do utilizador antes de chamar.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"content": {
					Type:        "STRING",
					Description: "Conteúdo da memória a guardar (texto descritivo)",
				},
				"valence": {
					Type:        "STRING",
					Description: "Valência emocional: número entre -1.0 (aversão) e 1.0 (preferência). Ex: '-0.7' para algo negativo, '0.8' para algo positivo.",
				},
				"confirm": {
					Type:        "BOOLEAN",
					Description: "OBRIGATÓRIO true — EVA deve anunciar o que vai guardar e receber confirmação antes de chamar esta tool",
				},
			},
			Required: []string{"content", "valence", "confirm"},
		},
	}
}

// ReorganizeThoughtsDefinition returns the schema for the reorganize_thoughts tool
func ReorganizeThoughtsDefinition() FunctionDeclaration {
	return FunctionDeclaration{
		Name:        "reorganize_thoughts",
		Description: "Acciona reconsolidação (sleep) numa collection não-core. Optimiza embeddings Poincaré via gradient descent. Assíncrono — EVA não bloqueia.",
		Parameters: &FunctionParameters{
			Type: "OBJECT",
			Properties: map[string]*Property{
				"target_collection": {
					Type:        "STRING",
					Description: "Collection a reorganizar (apenas não-core: culture_galaxies, eva_mind, eva_learnings, etc.)",
				},
			},
			Required: []string{"target_collection"},
		},
	}
}

// GetToolDefinitions returns all available tool definitions
func GetToolDefinitions() []FunctionDeclaration {
	return []FunctionDeclaration{
		GetVitalsDefinition(),
		GetAgendamentosDefinition(),
		ScanMedicationVisualDefinition(),
		AnalyzeVoiceProsodyDefinition(),
		ApplyPHQ9Definition(),
		ApplyGAD7Definition(),
		ApplyCSSRSDefinition(),
		SubmitPHQ9ResponseDefinition(),
		SubmitGAD7ResponseDefinition(),
		SubmitCSSRSResponseDefinition(),
		ChangeUserDirectiveDefinition(),
		// Proprioception tools
		BrainScanDefinition(),
		FeelTheGraphDefinition(),
		InternalizeMemoryDefinition(),
		ReorganizeThoughtsDefinition(),
	}
}
