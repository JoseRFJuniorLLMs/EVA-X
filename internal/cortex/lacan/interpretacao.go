// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"database/sql"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// InterpretationService coordena todos os servicos lacanianos
type InterpretationService struct {
	transferencia *TransferenceService
	significante  *SignifierService
	demandaDesejo *DemandDesireService
	grandAutre    *GrandAutreService

	db          *sql.DB
	graphClient *nietzscheInfra.GraphAdapter
}

// NewInterpretationService cria servico completo de interpretacao
func NewInterpretationService(db *sql.DB, graphClient *nietzscheInfra.GraphAdapter) *InterpretationService {
	return &InterpretationService{
		transferencia: NewTransferenceService(db),
		significante:  NewSignifierService(graphClient),
		demandaDesejo: NewDemandDesireService(),
		grandAutre:    NewGrandAutreService(),
		db:            db,
		graphClient:   graphClient,
	}
}

// InterpretationResult contem resultado completo da analise lacaniana
type InterpretationResult struct {
	// Transferencia
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

	// Orientacao Clinica Final
	ClinicalGuidance string
}

// AnalyzeUtterance realiza analise lacaniana completa de uma fala
func (i *InterpretationService) AnalyzeUtterance(ctx context.Context, idosoID int64, text string, previousText string) (*InterpretationResult, error) {
	result := &InterpretationResult{}

	// 1. Detectar transferencia
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

	// 4. Grande Outro: reflexao
	result.ReflexiveQuestion = i.grandAutre.ReflectiveQuestion(text)

	// 5. Detectar contradicao (se houver texto anterior)
	if previousText != "" {
		result.Contradiction = i.grandAutre.PointToContradiction(previousText, text)
	}

	// 6. Decidir sobre silencio
	result.ShouldSilence = i.grandAutre.IntentionalSilence(text)

	// 7. Montar orientacao clinica final
	result.ClinicalGuidance = i.buildClinicalGuidance(result)

	return result, nil
}

// buildClinicalGuidance monta orientacao clinica consolidada
func (i *InterpretationService) buildClinicalGuidance(result *InterpretationResult) string {
	guidance := "\nANALISE LACANIANA:\n\n"

	// Transferencia
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

	// Interpelacao
	if result.ShouldInterpel {
		guidance += "MOMENTO DE INTERPELACAO:\n"
		guidance += "- Use: \"" + result.InterpelPhrase + "\"\n\n"
	}

	// Contradicao
	if result.Contradiction != "" {
		guidance += "CONTRADICAO DETECTADA:\n"
		guidance += "- " + result.Contradiction + "\n\n"
	}

	// Reflexao
	guidance += "POSTURA RECOMENDADA:\n"
	if result.ShouldSilence {
		guidance += "- Faca uma pausa. De espaco para elaboracao.\n"
		guidance += "- Demonstre presenca atraves do silencio.\n"
	} else {
		guidance += "- Use pergunta reflexiva: \"" + result.ReflexiveQuestion + "\"\n"
		guidance += "- Nao ofereca solucoes imediatas. Deixe o sujeito elaborar.\n"
	}

	return guidance
}

// GetLacanianContext monta contexto para o prompt do Gemini
func (i *InterpretationService) GetLacanianContext(ctx context.Context, idosoID int64) string {
	// Buscar transferencia dominante
	transf, _ := i.transferencia.GetDominantTransference(ctx, idosoID)

	// Buscar significantes-chave
	signifiers, _ := i.significante.GetKeySignifiers(ctx, idosoID, 5)

	context := "\n---\nORIENTACOES PSICANALITICAS (LACAN)\n---\n\n"

	// Transferencia
	if transf != TRANSFERENCIA_NENHUMA {
		context += "TRANSFERENCIA DETECTADA: " + string(transf) + "\n"
		context += GetTransferenceGuidance(transf) + "\n"
	}

	// Significantes
	if len(signifiers) > 0 {
		context += "SIGNIFICANTES RECORRENTES:\n"
		for _, sig := range signifiers {
			context += "- '" + sig.Word + "' (apareceu " + string(rune(sig.Frequency)) + "x)\n"
		}
		context += "-> Preste atencao quando essas palavras aparecerem\n\n"
	}

	// Postura geral
	context += "POSTURA ANALITICA:\n"
	context += "1. ESCUTA FLUTUANTE: Nao se prenda ao conteudo manifesto\n"
	context += "2. ATENCAO AOS SIGNIFICANTES: Palavras que se repetem tem sentido inconsciente\n"
	context += "3. DISTINGUIR DEMANDA/DESEJO: O que o paciente pede != o que ele deseja\n"
	context += "4. DEVOLVER A FALA: 'Voce disse X... o que isso significa para voce?'\n"
	context += "5. NAO RESOLVER O IMPOSSIVEL: Trauma, morte, perda nao tem solucao. Ajude a simbolizar.\n"
	context += "6. ACOLHER TRANSFERENCIA: Se o paciente projeta afetos em voce, acolha sem negar\n"
	context += "\n---\n"

	return context
}
