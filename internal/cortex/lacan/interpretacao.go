// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"database/sql"
	"eva-mind/internal/brainstem/infrastructure/graph"
)

// InterpretationService coordena todos os serviços lacanianos
type InterpretationService struct {
	transferencia *TransferenceService
	significante  *SignifierService
	demandaDesejo *DemandDesireService
	grandAutre    *GrandAutreService // Corrected generic name if it was split

	db          *sql.DB
	neo4jClient *graph.Neo4jClient
}

// NewInterpretationService cria serviço completo de interpretação
func NewInterpretationService(db *sql.DB, neo4jClient *graph.Neo4jClient) *InterpretationService {
	return &InterpretationService{
		transferencia: NewTransferenceService(db),
		significante:  NewSignifierService(neo4jClient),
		demandaDesejo: NewDemandDesireService(),
		grandAutre:    NewGrandAutreService(),
		db:            db,
		neo4jClient:   neo4jClient,
	}
}

// InterpretationResult contém resultado completo da análise lacaniana
type InterpretationResult struct {
	// Transferência
	Transference     TransferenceType
	TransferGuidance string

	// Significantes
	KeySignifiers  []Signifier
	ShouldInterpel bool
	InterpelPhrase string

	// Demanda/Desejo
	DemandDesire      *Analysis
	SuggestedResponse string

	// Grande Outro
	ReflexiveQuestion string
	Contradiction     string
	ShouldSilence     bool

	// Orientação Clínica Final
	ClinicalGuidance string
}

// AnalyzeUtterance realiza análise lacaniana completa de uma fala
func (i *InterpretationService) AnalyzeUtterance(ctx context.Context, idosoID int64, text string, previousText string) (*InterpretationResult, error) {
	result := &InterpretationResult{}

	// 1. Detectar transferência
	transf := i.transferencia.DetectTransference(ctx, idosoID, text)
	result.Transference = transf
	result.TransferGuidance = GetTransferenceGuidance(transf)

	// 2. Rastrear significantes
	i.significante.TrackSignifiers(ctx, idosoID, text)
	signifiers, err := i.significante.GetKeySignifiers(ctx, idosoID, 5)
	if err == nil {
		result.KeySignifiers = signifiers

		// Verificar se deve interpelar algum significante
		for _, sig := range signifiers {
			should, _ := i.significante.ShouldInterpelSignifier(ctx, idosoID, sig.Word)
			if should {
				result.ShouldInterpel = true
				result.InterpelPhrase = GenerateInterpellation(sig.Word, sig.Frequency)
				i.significante.MarkAsInterpelled(ctx, idosoID, sig.Word)
				break
			}
		}
	}

	// 3. Analisar demanda/desejo
	analysis := i.demandaDesejo.AnalyzeUtterance(text)
	result.DemandDesire = analysis
	result.SuggestedResponse = i.demandaDesejo.GenerateResponse(analysis)

	// 4. Grande Outro: reflexão
	result.ReflexiveQuestion = i.grandAutre.ReflectiveQuestion(text)

	// 5. Detectar contradição (se houver texto anterior)
	if previousText != "" {
		result.Contradiction = i.grandAutre.PointToContradiction(previousText, text)
	}

	// 6. Decidir sobre silêncio
	result.ShouldSilence = i.grandAutre.IntentionalSilence(text)

	// 7. Montar orientação clínica final
	result.ClinicalGuidance = i.buildClinicalGuidance(result)

	return result, nil
}

// buildClinicalGuidance monta orientação clínica consolidada
func (i *InterpretationService) buildClinicalGuidance(result *InterpretationResult) string {
	guidance := "\n🧠 ANÁLISE LACANIANA:\n\n"

	// Transferência
	if result.Transference != TRANSFERENCIA_NENHUMA {
		guidance += result.TransferGuidance + "\n"
	}

	// Desejo Latente
	if result.DemandDesire.LatentDesire != DESEJO_INDEFINIDO {
		guidance += "DESEJO LATENTE: " + string(result.DemandDesire.LatentDesire) + "\n"
		guidance += "- " + result.DemandDesire.Interpretation + "\n"
		guidance += "- " + GetClinicalGuidance(result.DemandDesire.LatentDesire) + "\n\n"
	}

	// Significantes
	if len(result.KeySignifiers) > 0 {
		guidance += "SIGNIFICANTES RECORRENTES:\n"
		for _, sig := range result.KeySignifiers {
			guidance += "- '" + sig.Word + "' (" + string(rune(sig.Frequency)) + "x)\n"
		}
		guidance += "\n"
	}

	// Interpelação
	if result.ShouldInterpel {
		guidance += "⚠️ MOMENTO DE INTERPELAÇÃO:\n"
		guidance += "- Use: \"" + result.InterpelPhrase + "\"\n\n"
	}

	// Contradição
	if result.Contradiction != "" {
		guidance += "⚡ CONTRADIÇÃO DETECTADA:\n"
		guidance += "- " + result.Contradiction + "\n\n"
	}

	// Reflexão
	guidance += "POSTURA RECOMENDADA:\n"
	if result.ShouldSilence {
		guidance += "- Faça uma pausa. Dê espaço para elaboração.\n"
		guidance += "- Demonstre presença através do silêncio.\n"
	} else {
		guidance += "- Use pergunta reflexiva: \"" + result.ReflexiveQuestion + "\"\n"
		guidance += "- Não ofereça soluções imediatas. Deixe o sujeito elaborar.\n"
	}

	return guidance
}

// GetLacanianContext monta contexto para o prompt do Gemini
func (i *InterpretationService) GetLacanianContext(ctx context.Context, idosoID int64) string {
	// Buscar transferência dominante
	transf, _ := i.transferencia.GetDominantTransference(ctx, idosoID)

	// Buscar significantes-chave
	signifiers, _ := i.significante.GetKeySignifiers(ctx, idosoID, 5)

	context := "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
	context += "🧠 ORIENTAÇÕES PSICANALÍTICAS (LACAN)\n"
	context += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"

	// Transferência
	if transf != TRANSFERENCIA_NENHUMA {
		context += "TRANSFERÊNCIA DETECTADA: " + string(transf) + "\n"
		context += GetTransferenceGuidance(transf) + "\n"
	}

	// Significantes
	if len(signifiers) > 0 {
		context += "SIGNIFICANTES RECORRENTES:\n"
		for _, sig := range signifiers {
			context += "- '" + sig.Word + "' (apareceu " + string(rune(sig.Frequency)) + "x)\n"
		}
		context += "→ Preste atenção quando essas palavras aparecerem\n\n"
	}

	// Postura geral
	context += "POSTURA ANALÍTICA:\n"
	context += "1. ESCUTA FLUTUANTE: Não se prenda ao conteúdo manifesto\n"
	context += "2. ATENÇÃO AOS SIGNIFICANTES: Palavras que se repetem têm sentido inconsciente\n"
	context += "3. DISTINGUIR DEMANDA/DESEJO: O que o paciente pede ≠ o que ele deseja\n"
	context += "4. DEVOLVER A FALA: 'Você disse X... o que isso significa para você?'\n"
	context += "5. NÃO RESOLVER O IMPOSSÍVEL: Trauma, morte, perda não têm solução. Ajude a simbolizar.\n"
	context += "6. ACOLHER TRANSFERÊNCIA: Se o paciente projeta afetos em você, acolha sem negar\n"
	context += "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"

	return context
}
