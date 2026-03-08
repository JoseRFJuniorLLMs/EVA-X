// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// seed_knowledge populates eva_self_knowledge in NietzscheDB AND NietzscheDB vector (semantic search).
// Covers ALL 130+ tables, all modules, all concepts, all infrastructure.
// Run: go run cmd/seed_knowledge/main.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"eva/internal/brainstem/config"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/hippocampus/knowledge"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type KnowledgeEntry struct {
	Type       string
	Key        string
	Title      string
	Summary    string
	Content    string
	Location   string
	Parent     string
	Tags       string
	Importance int
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:Debian23%40@34.35.142.107:5432/eva-mind?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("DB connect failed: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS eva_self_knowledge (
			id SERIAL PRIMARY KEY,
			knowledge_type VARCHAR(100) NOT NULL,
			knowledge_key VARCHAR(300) NOT NULL UNIQUE,
			title VARCHAR(500) NOT NULL,
			summary TEXT NOT NULL,
			detailed_content TEXT NOT NULL,
			code_location VARCHAR(500),
			parent_key VARCHAR(300),
			related_keys JSONB DEFAULT '[]',
			tags JSONB DEFAULT '[]',
			importance INTEGER DEFAULT 5,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Fatalf("Table creation failed: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Printf("WARN: Config load failed (NietzscheDB vector disabled): %v", err)
	}

	var vectorAdapter *nietzscheInfra.VectorAdapter
	var embedSvc *knowledge.EmbeddingService
	if cfg != nil {
		nietzscheClient, niErr := nietzscheInfra.NewClient(cfg.NietzscheGRPCAddr)
		if niErr != nil {
			log.Printf("WARN: NietzscheDB unavailable: %v", niErr)
		} else {
			defer nietzscheClient.Close()
			vectorAdapter = nietzscheInfra.NewVectorAdapter(nietzscheClient)
			embedSvc, err = knowledge.NewEmbeddingService(cfg, vectorAdapter)
			if err != nil {
				log.Printf("WARN: Embedding service unavailable: %v", err)
			}
		}
	}

	ctx := context.Background()
	entries := getAllEntries()

	// 1. NietzscheDB
	stmt, err := db.Prepare(`
		INSERT INTO eva_self_knowledge (knowledge_type, knowledge_key, title, summary, detailed_content, code_location, parent_key, tags, importance, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (knowledge_key) DO UPDATE SET
			title = EXCLUDED.title,
			summary = EXCLUDED.summary,
			detailed_content = EXCLUDED.detailed_content,
			code_location = EXCLUDED.code_location,
			parent_key = EXCLUDED.parent_key,
			tags = EXCLUDED.tags,
			importance = EXCLUDED.importance,
			updated_at = NOW()
	`)
	if err != nil {
		log.Fatalf("Prepare failed: %v", err)
	}
	defer stmt.Close()

	pgCount := 0
	for i, e := range entries {
		_, err := stmt.Exec(e.Type, e.Key, e.Title, e.Summary, e.Content, e.Location, e.Parent, e.Tags, e.Importance)
		if err != nil {
			log.Printf("WARN [%d] PG %s: %v", i, e.Key, err)
		} else {
			pgCount++
		}
	}
	fmt.Printf("NietzscheDB: %d/%d entries seeded\n", pgCount, len(entries))

	// 2. NietzscheDB vector index
	if vectorAdapter != nil && embedSvc != nil {
		collName := "eva_self_knowledge"
		nietzscheCount := 0

		for i, e := range entries {
			text := fmt.Sprintf("%s: %s\n%s", e.Title, e.Summary, e.Content)
			if len(text) > 4000 {
				text = text[:4000]
			}

			embedding, err := embedSvc.GenerateEmbedding(ctx, text)
			if err != nil {
				log.Printf("WARN [%d] Embed %s: %v", i, e.Key, err)
				continue
			}

			pointID := fmt.Sprintf("%d-%d", time.Now().UnixNano()/1000000, i)
			payload := map[string]interface{}{
				"key":        e.Key,
				"type":       e.Type,
				"title":      e.Title,
				"summary":    e.Summary,
				"content":    e.Content,
				"location":   e.Location,
				"importance": int64(e.Importance),
			}

			if err := vectorAdapter.Upsert(ctx, collName, pointID, embedding, payload); err != nil {
				log.Printf("WARN Upsert [%d] %s: %v", i, e.Key, err)
			} else {
				nietzscheCount++
			}

			if (i+1)%3 == 0 {
				time.Sleep(100 * time.Millisecond)
			}
		}

		fmt.Printf("NietzscheDB: %d/%d entries indexed in '%s'\n", nietzscheCount, len(entries), collName)
	} else {
		fmt.Println("NietzscheDB: SKIPPED (unavailable)")
	}
}

func getAllEntries() []KnowledgeEntry {
	var e []KnowledgeEntry
	e = append(e, architectureEntries()...)
	e = append(e, databaseEntries()...)
	e = append(e, moduleEntries()...)
	e = append(e, conceptEntries()...)
	e = append(e, apiEntries()...)
	e = append(e, infraEntries()...)
	return e
}

// ======================== ARCHITECTURE ========================

func architectureEntries() []KnowledgeEntry {
	return []KnowledgeEntry{
		{
			Type: "architecture", Key: "arch:overview", Title: "Arquitetura Geral do EVA-Mind",
			Summary: "EVA-Mind: IA companeira para idosos. Voz em tempo real, 12 agentes swarm, 130+ tabelas, 110+ tools, 2 bancos (NietzscheDB, NietzscheDB)",
			Content: `EVA-Mind e um sistema de IA companeira para cuidar de idosos, com arquitetura inspirada no cerebro humano.

CAMADAS:
1. BRAINSTEM — Infraestrutura: config, database (NietzscheDB), NietzscheDB (graph + vector + cache), auth (JWT), push (Firebase)
2. CORTEX — Logica e IA: gemini (voz bidirecional + tools), lacan (psicanalise), personality (Eneagrama 9 tipos), learning (estudo autonomo), self (Core Memory), selfawareness (introspecao), eva_memory (meta-cognitiva), alert (escalation), voice/speaker (fingerprinting)
3. HIPPOCAMPUS — Memoria: memory (episodica + graph + retrieval), superhuman (12 subsistemas), knowledge (embeddings + wisdom + self-knowledge), habits (tracking), spaced (repetition SM-2)
4. MOTOR — Acoes: email (SMTP)
5. SWARM — 12 agentes: clinical, emergency, entertainment, wellness, productivity, google, external, educator, kids, legal, scholar, selfawareness
6. TOOLS — 110+ ferramentas
7. VOICE — WebSocket voz tempo real (PCM 16kHz in, 24kHz out)
8. SCHEDULER — Background jobs
9. SECURITY — CORS middleware

BANCOS:
- NietzscheDB 15: 130+ tabelas (34.35.142.107:5432)
- NietzscheDB: Grafo multi-manifold + vetores hiperbolicos (gRPC :50051)

FASES DE IMPLEMENTACAO (7 completas):
E0: Situational Modulator, A: Hebbian Real-Time, B: FDPN Boost, C: Edge Zones, D: Entity Resolution, E: RAM (Realistic Accuracy Model), F: Core Memory`,
			Location: "main.go", Tags: `["arquitetura", "visao_geral"]`, Importance: 10,
		},
		{
			Type: "architecture", Key: "arch:project_structure", Title: "Estrutura de Diretorios do Projeto",
			Summary: "380+ arquivos .go, 27 arquivos .md, 41+ migrations SQL. Organizacao por cerebro humano: brainstem, cortex, hippocampus, motor, swarm",
			Content: `eva-mind/
├── main.go                          # Entry point — wiring de todos os servicos
├── browser_voice_handler.go         # WebSocket handler browser/app (Gemini Live + tools + reconexao)
├── eva_chat_handler.go              # Chat por texto
├── video_handler.go                 # Video WebRTC signaling
├── log_stream_handler.go            # Stream de logs
├── internal/
│   ├── brainstem/                   # Infraestrutura
│   │   ├── auth/                    # JWT
│   │   ├── config/                  # .env
│   │   ├── database/                # NietzscheDB
│   │   ├── infrastructure/graph/    # NietzscheDB
│   │   └── infrastructure/vector/   # NietzscheDB
│   │   └── push/                    # Firebase
│   ├── cortex/                      # Logica e IA
│   │   ├── alert/                   # Escalation
│   │   ├── eva_memory/              # Meta-cognitive
│   │   ├── gemini/                  # Gemini Live + Flash + Tools
│   │   ├── lacan/                   # FDPN, Narrative Shift, Signifiers, Unified Retrieval
│   │   ├── learning/                # Autonomous Learner
│   │   ├── personality/             # Eneagrama
│   │   ├── self/                    # Core Memory, Reflection, Anonymization
│   │   ├── selfawareness/           # Introspecao (AST parser, busca semantica)
│   │   ├── situation/               # Situational Modulator
│   │   └── voice/speaker/           # Voice fingerprinting ECAPA-TDNN
│   ├── hippocampus/                 # Memoria
│   │   ├── habits/                  # Habit tracking
│   │   ├── knowledge/               # Embeddings, Wisdom, Self-Knowledge
│   │   ├── memory/                  # Episodic + Graph + Retrieval + Hebbian
│   │   │   └── superhuman/          # 12 subsistemas
│   │   └── spaced/                  # Spaced repetition SM-2
│   ├── motor/email/                 # SMTP
│   ├── scheduler/                   # Background jobs
│   ├── security/                    # CORS
│   ├── swarm/ (12 agents)           # Multi-agent system
│   ├── telemetry/                   # Zerolog
│   ├── tools/                       # 110+ tool handlers
│   └── voice/                       # Session management
├── cmd/
│   ├── index_code/                  # Indexador de codigo (AST) + docs (.md) → NietzscheDB
│   ├── seed_knowledge/              # Seed de conhecimento → NietzscheDB + NietzscheDB
│   └── seed_wisdom/                 # Seed de sabedoria → NietzscheDB
├── migrations/ (41+ SQL)            # Todas as tabelas
├── MD/ (27 .md files)               # Documentacao de arquitetura e fases
└── sabedoria/conhecimento/          # Textos de sabedoria`,
			Location: ".", Tags: `["estrutura", "diretorios"]`, Importance: 9,
		},
	}
}

// ======================== DATABASES ========================

func databaseEntries() []KnowledgeEntry {
	return []KnowledgeEntry{
		// --- NietzscheDB Overview ---
		{
			Type: "database", Key: "db:NietzscheDB", Title: "NietzscheDB — Banco Principal (130+ tabelas)",
			Summary: "NietzscheDB 15 em 34.35.142.107:5432. 130+ tabelas cobrindo pacientes, clinica, memoria, personalidade, tools, pesquisa, etica, Lacan, speaker",
			Content: `NietzscheDB e o banco principal. Host: 34.35.142.107:5432, DB: eva-mind, User: postgres.

DOMINIOS DE TABELAS (130+ tabelas em 41 migrations):
1. PACIENTES & AGENDAMENTO (4 tabelas): idosos, agendamentos, historico_ligacoes, device_tokens
2. CLINICA & ASSESSMENT (6): clinical_assessments, clinical_assessment_responses, medication_visual_logs, medication_identifications, voice_prosody_analyses, voice_prosody_features
3. CARGA COGNITIVA & ETICA (6): interaction_cognitive_load, cognitive_load_state, cognitive_load_decisions, ethical_boundary_events, ethical_boundary_state, ethical_redirections
4. DECISAO CLINICA EXPLICAVEL (3): clinical_decision_explanations, decision_factors, prediction_accuracy_log
5. TRAJETORIA PREDITIVA (5): trajectory_simulations, intervention_scenarios, recommended_interventions, trajectory_prediction_accuracy, bayesian_network_parameters
6. PESQUISA CLINICA (6): research_cohorts, research_datapoints, longitudinal_correlations, statistical_analyses, research_publications, research_exports
7. MULTI-PERSONA (5): persona_definitions, persona_sessions, persona_activation_rules, persona_tool_permissions, persona_transitions
8. PROTOCOLO DE SAIDA & PALIATIVO (7): last_wishes, quality_of_life_assessments, pain_symptom_logs, legacy_messages, farewell_preparation, comfort_care_plans, spiritual_care_sessions
9. INTEGRACAO API (8): api_clients, api_tokens, api_request_logs, webhook_deliveries, rate_limit_tracking, fhir_resource_mappings, external_system_credentials, data_export_jobs
10. ESCALATION (3): escalation_logs, escalation_attempts, emergency_contacts
11. SUPERHUMAN MEMORY (25+): enneagram_types, patient_enneagram, enneagram_evidence, patient_self_core, patient_master_signifiers, patient_behavioral_patterns, patient_circadian_patterns, patient_intentions, patient_counterfactuals, patient_metaphors, patient_family_patterns, patient_somatic_correlations, patient_cultural_context, patient_effective_approaches, patient_optimal_silence, patient_crisis_predictors, patient_risk_scores, patient_world_persons, patient_world_places, patient_world_objects, patient_persistent_memories, persistent_memory_occurrences, patient_place_transitions, patient_place_sensory_memories, patient_shared_memories, patient_undelivered_messages, patient_body_memories, body_memory_occurrences, patient_narrative_threads, patient_life_markers
12. CONSCIENCIA (15+): patient_memory_gravity, patient_cycle_patterns, cycle_pattern_occurrences, patient_rapport, rapport_events, patient_narrative_versions, patient_contradiction_summary, patient_eva_mode, mode_transitions, patient_relationship_evolution, patient_error_memory, patient_empathic_load, empathic_load_events, patient_intervention_readiness
13. MEMORIA CRITICA (4): patient_memory_clusters, cluster_members, forgotten_memories, patient_temporal_config
14. AUDITORIA ETICA (2): ethical_audit_rules, ethical_audit_log
15. TOOLS DINAMICO (4): available_tools, tool_invocation_log, tool_permissions, eva_capabilities
16. EVA SELF-KNOWLEDGE (2): eva_self_knowledge, creator_knowledge_access
17. APRENDIZAGEM (2): eva_curriculum, eva_self_knowledge
18. LACAN (9): lacan_transferencia_patterns, lacan_transferencia_guidance, lacan_desire_patterns, lacan_desire_responses, lacan_emotional_keywords, lacan_addressee_patterns, lacan_addressee_guidance, lacan_elaboration_markers, lacan_ethical_principles
19. SPEAKER (2): speaker_profiles, speaker_identifications
20. ESTILO CONVERSA (1+): estilos_conversa_config (+ campos adicionais em idosos)
21. HABITOS & SPACED (3): habits_log, spaced_repetition_items, episodic_memories
22. KIDS & GTD (3+): kid_missions, gtd_tasks`,
			Location: "migrations/", Tags: `["NietzscheDB", "tabelas", "esquema"]`, Importance: 10,
		},
		// --- Tables: Pacientes ---
		{
			Type: "database", Key: "db:tables:pacientes", Title: "Tabelas de Pacientes e Agendamento",
			Summary: "idosos (dados do paciente + config), agendamentos (compromissos), historico_ligacoes (chamadas), device_tokens (push)",
			Content: `TABELA idosos:
id SERIAL PK, nome, telefone, nivel_cognitivo, limitacoes_auditivas, estilo_conversa VARCHAR(20) DEFAULT 'hibrido', persona_preferida DEFAULT 'companion', profundidade_emocional DECIMAL DEFAULT 0.70, created_at, updated_at

TABELA agendamentos:
id, idoso_id FK, nome_idoso, telefone, horario, remedios, status, tentativas_realizadas, call_sid, gemini_session_handle, ultima_interacao_estado, created_at, updated_at

TABELA historico_ligacoes:
id, agendamento_id, idoso_id, call_sid, status, inicio, fim, qualidade_audio, interrupcoes_detectadas, created_at

TABELA device_tokens:
id, idoso_id FK, token, platform (ios/android), app_version, device_model, is_active, created_at, last_used_at`,
			Location: "migrations/001", Parent: "db:NietzscheDB", Tags: `["pacientes", "agendamentos"]`, Importance: 8,
		},
		// --- Tables: Clinical ---
		{
			Type: "database", Key: "db:tables:clinical", Title: "Tabelas Clinicas (Assessments + Medication + Voice Prosody)",
			Summary: "Avaliacoes PHQ-9/GAD-7/C-SSRS/MMSE/MoCA, identificacao visual de medicamentos, analise de prosodia vocal (biomarkers)",
			Content: `TABELA clinical_assessments:
id UUID PK, patient_id, assessment_type (phq9/gad7/cssrs/mmse/moca), session_id, status, total_score, severity_level, trigger_phrase, priority, clinical_interpretation, alert_sent, created_at, completed_at

TABELA clinical_assessment_responses:
id, assessment_id FK, question_number, question_text, response_value, response_text, responded_at

TABELA medication_visual_logs:
id, patient_id, session_id, scan_status, confidence_score, gemini_model_used, error_message, created_at

TABELA medication_identifications:
id, visual_log_id FK, medication_name, dosage, pharmaceutical_form, pill_color, manufacturer, confidence, matched_medication_id, safety_status, safety_warnings, created_at

TABELA voice_prosody_analyses:
id, patient_id, session_id, analysis_type, audio_duration_seconds, transcript, gemini_model_used, depression_risk_score, anxiety_risk_score, parkinson_risk_score, hydration_score, overall_assessment, clinical_flags, alert_sent, created_at

TABELA voice_prosody_features:
id, analysis_id FK, pitch_mean, pitch_std, pitch_min, pitch_max, jitter, shimmer, hnr, speech_rate, pause_duration_mean, pause_frequency, monotonicity_score, tremor_indicator, breathlessness_score, created_at`,
			Location: "migrations/002-003", Parent: "db:NietzscheDB", Tags: `["clinica", "assessment", "medicamentos", "prosodia"]`, Importance: 9,
		},
		// --- Tables: Cognitive Load & Ethics ---
		{
			Type: "database", Key: "db:tables:cognitive_ethics", Title: "Carga Cognitiva e Limites Eticos",
			Summary: "Rastreamento de carga cognitiva por interacao, estado etico, eventos de boundary violation, redirecionamentos",
			Content: `TABELA interaction_cognitive_load:
id UUID, patient_id, timestamp, interaction_type, emotional_intensity, cognitive_complexity, duration_seconds, patient_fatigue_indicators, topics_discussed, lacanian_signifiers, session_id, voice_energy_score, speech_rate_wpm, pause_frequency, cumulative_load_24h

TABELA cognitive_load_state:
patient_id PK, current_load_score, load_24h, load_7d, interactions_count_24h, therapeutic_count_24h, high_intensity_count_24h, last_interaction_at, rumination_detected, emotional_saturation, fatigue_level, active_restrictions, restriction_until

TABELA cognitive_load_decisions:
id UUID, patient_id, timestamp, current_load, trigger_event, decision_type, blocked_actions, allowed_actions, redirect_suggestion, tone_adjustment

TABELA ethical_boundary_events:
id UUID, patient_id, event_type, severity, evidence, trigger_phrase, attachment_indicators_count, eva_vs_human_ratio, action_taken, redirection_attempted, family_notified

TABELA ethical_boundary_state:
patient_id PK, attachment_risk_score, isolation_risk_score, dependency_risk_score, overall_ethical_risk, eva_interactions_7d, human_interactions_7d, eva_vs_human_ratio, active_ethical_limits

TABELA ethical_redirections:
id UUID, patient_id, event_id, trigger_reason, severity_level, redirection_level, strategy_used, eva_message, patient_response, compliance_achieved`,
			Location: "migrations/003", Parent: "db:NietzscheDB", Tags: `["cognitiva", "etica", "limites", "carga"]`, Importance: 8,
		},
		// --- Tables: Prediction ---
		{
			Type: "database", Key: "db:tables:prediction", Title: "Trajetoria Preditiva (Monte Carlo + Bayesian)",
			Summary: "Simulacoes Monte Carlo de trajetorias, cenarios de intervencao, recomendacoes, acuracia, rede Bayesiana",
			Content: `TABELA trajectory_simulations:
id UUID, patient_id, simulation_date, days_ahead, n_simulations, crisis_probability_7d, crisis_probability_30d, hospitalization_probability_30d, treatment_dropout_probability_90d, fall_risk_probability_7d, projected_phq9_score, model_version, computation_time_ms

TABELA intervention_scenarios:
id UUID, simulation_id FK, patient_id, scenario_type, scenario_name, interventions JSONB, crisis_probability_7d, risk_reduction_7d, effectiveness_score, estimated_cost_monthly, feasibility

TABELA recommended_interventions:
id UUID, simulation_id FK, patient_id, intervention_type, priority, urgency_timeframe, title, description, rationale, expected_risk_reduction, status, implemented_at

TABELA trajectory_prediction_accuracy:
id UUID, simulation_id FK, predicted_crisis_7d, actual_crisis_occurred, prediction_correct, false_positive, false_negative, calibration_score

TABELA bayesian_network_parameters:
id UUID, model_version, node_name, node_type, parent_nodes, conditional_probability_table, confidence_interval, cross_validation_score, auc_roc`,
			Location: "migrations/005", Parent: "db:NietzscheDB", Tags: `["predicao", "monte_carlo", "bayesian", "trajetoria"]`, Importance: 8,
		},
		// --- Tables: Research ---
		{
			Type: "database", Key: "db:tables:research", Title: "Motor de Pesquisa Clinica (Cohorts + Publications)",
			Summary: "Estudos cientificos, datapoints anonimizados, correlacoes longitudinais, analises estatisticas, publicacoes com DOI",
			Content: `TABELA research_cohorts:
id UUID, study_name, study_code, hypothesis, study_type, inclusion_criteria, target_n_patients, status, primary_outcome, statistical_methods, p_value, effect_size, paper_title, journal, ethics_committee_approval

TABELA research_datapoints:
id UUID, cohort_id FK, anonymous_patient_id, observation_date, phq9_score, gad7_score, cssrs_score, medication_adherence_7d, sleep_hours_avg_7d, voice_pitch_mean_hz, speech_rate_wpm, social_isolation_days, crisis_occurred, is_anonymized

TABELA longitudinal_correlations:
id UUID, cohort_id FK, predictor_variable, outcome_variable, lag_days, correlation_coefficient, p_value, is_significant, effect_size_category

TABELA statistical_analyses:
id UUID, cohort_id FK, analysis_type, analysis_name, independent_variables, dependent_variable, p_value, is_significant, model_fit_metrics, cv_mean_score

TABELA research_publications:
id UUID, cohort_id FK, title, abstract, authors, journal_name, doi, pmid, citations_count, status

TABELA research_exports:
id UUID, cohort_id FK, export_format, anonymization_level, lgpd_compliant, hipaa_compliant`,
			Location: "migrations/007", Parent: "db:NietzscheDB", Tags: `["pesquisa", "cohort", "publicacao", "estatistica"]`, Importance: 7,
		},
		// --- Tables: Persona ---
		{
			Type: "database", Key: "db:tables:persona", Title: "Sistema Multi-Persona",
			Summary: "Definicoes de persona (voz, tom, profundidade), sessoes, regras de ativacao automatica, permissoes de tools, transicoes",
			Content: `TABELA persona_definitions:
id UUID, persona_code, persona_name, description, voice_id, tone, emotional_depth, narrative_freedom, max_session_duration_minutes, max_daily_interactions, max_intimacy_level, require_professional_oversight, allowed_tools JSONB, prohibited_tools JSONB, system_instruction_template, is_active

TABELA persona_sessions:
id UUID, patient_id, persona_code, started_at, ended_at, trigger_reason, triggered_by, tools_used, boundary_violations, escalation_required, patient_feedback_rating

TABELA persona_activation_rules:
id UUID, rule_name, target_persona_code, priority, conditions JSONB, auto_activate, cooldown_hours, is_active

TABELA persona_tool_permissions:
id UUID, persona_code, tool_name, permission_type, max_uses_per_day, requires_reason, emergency_override_allowed

TABELA persona_transitions:
id UUID, patient_id, from_persona_code, to_persona_code, transition_at, reason, transition_successful`,
			Location: "migrations/008", Parent: "db:NietzscheDB", Tags: `["persona", "multipersonalidade", "ativacao"]`, Importance: 7,
		},
		// --- Tables: Exit Protocol ---
		{
			Type: "database", Key: "db:tables:exit_protocol", Title: "Protocolo de Saida e Cuidados Paliativos",
			Summary: "Diretivas antecipadas (last_wishes), qualidade de vida WHOQOL-BREF, log de dor/sintomas, mensagens de legado, preparacao de despedida, planos de conforto, sessoes espirituais",
			Content: `TABELA last_wishes: Diretivas antecipadas digitais
id UUID, patient_id, resuscitation_preference, mechanical_ventilation, artificial_nutrition, preferred_death_location, pain_management_preference, organ_donation_preference, funeral_preferences, personal_statement, completed, legally_binding

TABELA quality_of_life_assessments: WHOQOL-BREF
id UUID, patient_id, physical_domain_score, psychological_domain_score, social_domain_score, environmental_domain_score, overall_qol_score

TABELA pain_symptom_logs: Rastreamento diario
id UUID, patient_id, pain_intensity (0-10), pain_location, nausea_vomiting, shortness_of_breath, fatigue, anxiety_level, depression_level, overall_wellbeing

TABELA legacy_messages: Mensagens para entes queridos
id UUID, patient_id, recipient_name, recipient_relationship, message_type (text/audio/video), delivery_trigger, is_complete, has_been_delivered

TABELA farewell_preparation: Rastreamento de preparacao
id UUID, patient_id, legal_affairs_complete, financial_affairs_complete, reconciliations_completed, five_stages_grief_position, emotional_readiness, peace_with_death, overall_preparation_score

TABELA comfort_care_plans: Planos de manejo de sintomas
id UUID, patient_id, trigger_symptom, interventions JSONB, auto_activate, average_effectiveness

TABELA spiritual_care_sessions: Sessoes de aconselhamento espiritual
id UUID, patient_id, conducted_by, topics_discussed, existential_questions, pre_session_peace_level, post_session_peace_level`,
			Location: "migrations/009", Parent: "db:NietzscheDB", Tags: `["paliativo", "saida", "legado", "espiritual", "qualidade_vida"]`, Importance: 7,
		},
		// --- Tables: API Integration ---
		{
			Type: "database", Key: "db:tables:api_integration", Title: "Camada de Integracao API (OAuth2 + FHIR + Webhooks)",
			Summary: "Clientes OAuth2, tokens, log de requests, webhooks, rate limiting, mapeamento FHIR HL7, credenciais externas, exports LGPD",
			Content: `TABELA api_clients: Clientes OAuth2
id UUID, client_name, client_type, client_id, client_secret_hash, scopes, rate_limit_per_minute, webhook_url, is_active

TABELA api_tokens: Access/refresh tokens
id UUID, client_id FK, access_token, refresh_token, scopes, expires_at, is_revoked

TABELA api_request_logs: Audit log
id BIGSERIAL, request_id UUID, client_id, http_method, endpoint, http_status_code, response_time_ms, patient_id

TABELA webhook_deliveries: Fila de webhooks
id UUID, client_id, event_type, event_data JSONB, status, attempts, delivered_at

TABELA rate_limit_tracking: Controle de rate
id BIGSERIAL, client_id, window_type, request_count

TABELA fhir_resource_mappings: Interoperabilidade HL7 FHIR
id UUID, eva_resource_type, fhir_resource_type, fhir_resource JSONB, sync_status

TABELA external_system_credentials: Integracoes externas
id UUID, system_name, system_type, base_url, auth_type, credentials_encrypted, is_active

TABELA data_export_jobs: Exports LGPD/pesquisa
id UUID, patient_id, export_type, status, lgpd_compliant, anonymization_level`,
			Location: "migrations/010", Parent: "db:NietzscheDB", Tags: `["api", "oauth2", "fhir", "webhook", "lgpd"]`, Importance: 7,
		},
		// --- Tables: Superhuman Memory ---
		{
			Type: "database", Key: "db:tables:superhuman", Title: "Superhuman Memory — 30+ Tabelas de Memoria Profunda",
			Summary: "Eneagrama do paciente, self-core, signifiers mestres, padroes comportamentais, circadianos, intencoes, contrafactuais, metaforas, padroes familiares, somaticomemoria, contexto cultural, abordagens eficazes, silencio otimo, preditores de crise, scores de risco, mundo do paciente (pessoas/lugares/objetos)",
			Content: `TABELAS DE PERSONALIDADE:
- enneagram_types: 9 tipos com nome, centro, emocao_raiz, mecanismo_defesa, virtude, direcao_integracao/desintegracao
- patient_enneagram: Perfil do paciente (primary_type, confidence, wing, health_level, instinctual_variant)
- enneagram_evidence: Evidencias para tipagem (verbatim, suggested_type, weight, context)

TABELAS DE IDENTIDADE:
- patient_self_core: Auto-descricoes, papeis atribuidos, resumo narrativo, timeline de autoconceito
- patient_master_signifiers: Palavras/frases recorrentes (significantes-chave), contagem, co-ocorrencias

TABELAS COMPORTAMENTAIS:
- patient_behavioral_patterns: Padroes (tipo, trigger, resposta tipica, probabilidade, intervencoes eficazes)
- patient_circadian_patterns: Ritmos diarios (periodo, dia da semana, temas recorrentes, tom emocional)
- patient_intentions: Intencoes declaradas vs realidade (verbatim, status, bloqueio, completado_em)
- patient_counterfactuals: Ruminacoes "e se" (periodo de vida, tema, variancia de pitch, tremor de voz)
- patient_metaphors: Metaforas pessoais (tipo: corporal/espacial/temporal/relacional/existencial, contextos)

TABELAS FAMILIARES/CULTURAIS:
- patient_family_patterns: Padroes transgeracionais (comportamento herdado, mandato familiar, trauma, segredo)
- patient_cultural_context: Contexto geracional (ano nascimento, regiao, eventos historicos, valores, expressoes)

TABELAS TERAPEUTICAS:
- patient_effective_approaches: O que funciona (tipo, descricao, metrica, score, condicoes)
- patient_optimal_silence: Duracao otima de silencio por contexto (segundos, eficacia)
- patient_crisis_predictors: Marcadores preditivos de crise (tipo, preditor, peso, lead_time_days, acuracia)
- patient_risk_scores: Scores de risco em tempo real (depressao, suicidio 30d, hospitalizacao 90d, isolamento)

TABELAS DO MUNDO DO PACIENTE:
- patient_world_persons: Pessoas (nome, papel, valencia emocional, topicos associados, timeline do relacionamento)
- patient_world_places: Lugares (tipo, valencia, periodo temporal, memorias sensoriais, anos vividos)
- patient_world_objects: Objetos significativos (significancia descrita, pessoa associada, evento)`,
			Location: "migrations/012", Parent: "db:NietzscheDB", Tags: `["superhuman", "memoria", "eneagrama", "identidade", "padroes"]`, Importance: 9,
		},
		// --- Tables: Deep Memory ---
		{
			Type: "database", Key: "db:tables:deep_memory", Title: "Memoria Profunda — Persistencias, Transicoes, Corpo, Narrativas",
			Summary: "Memorias persistentes (traumas), transicoes de lugar, memorias sensoriais, memorias compartilhadas, mensagens nao entregues, memorias corporais, fios narrativos, marcos de vida",
			Content: `MEMORIAS PERSISTENTES (traumas que retornam):
- patient_persistent_memories: topico, keywords, tentativas_evitacao, frases_evitacao, contagem_retorno, distress prosodico, triggers, datas_aniversario, score de persistencia/evitacao
- persistent_memory_occurrences: tipo (mention/avoidance/return/triggered/spontaneous), verbatim, tremor de voz, pausa

LUGAR E SENSORIAL:
- patient_place_transitions: de_lugar → para_lugar, ano, idade, razao, impacto, tipo (voluntary/forced/family/health/loss), nostalgia
- patient_place_sensory_memories: tipo_sensorial (smell/sound/taste/touch/visual/temperature), descricao, emocoes, pessoas

COMPARTILHAMENTO:
- patient_shared_memories: resumo, audiencia_pretendida, status (wishes_to_share/partially_shared/fully_shared), tipo (life_lesson/family_history/love_story), urgencia
- patient_undelivered_messages: destinatario, relacionamento, essencia_mensagem, status (unspoken/attempted/delivered/impossible), bloqueio

CORPO-MENTE:
- patient_body_memories: sintoma fisico, localizacao_corpo, topicos correlacionados, forca_correlacao, paciente_consciente, cleared_medicamente
- body_memory_occurrences: verbatim, hora_do_dia, topicos_precedentes, intensidade (1-10)

NARRATIVA:
- patient_narrative_threads: elementos conectados, timeline, tipo_conexao (causal/emotional/temporal/person_topic), perguntas geradas
- patient_life_markers: descricao, ano, idade, tipo (birth/death/marriage/loss/trauma/achievement), impacto, antes/depois`,
			Location: "migrations/013", Parent: "db:NietzscheDB", Tags: `["memoria_profunda", "trauma", "corpo", "narrativa", "sensorial"]`, Importance: 8,
		},
		// --- Tables: Consciousness ---
		{
			Type: "database", Key: "db:tables:consciousness", Title: "Sistemas de Consciencia — Gravidade, Ciclos, Rapport, Narrativa, Empatia",
			Summary: "Gravidade de memoria, ciclos comportamentais mecanicos, rapport/confianca, versoes narrativas contradicoes, modos EVA (terapeuta/juiz/testemunha), evolucao relacional, memoria de erros, carga empatica",
			Content: `GRAVIDADE DE MEMORIA:
- patient_memory_gravity: gravity_score, emotional_valence, arousal, recall_frequency, identity_connection, pull_radius, collision_risk

CICLOS MECANICOS:
- patient_cycle_patterns: assinatura, tipo, contagem_ciclos, threshold, triggers, acoes, consequencias, intervencao_tentada, usuario_consciente
- cycle_pattern_occurrences: trigger, acao, consequencia, humor_antes/depois, fase_ciclo

RAPPORT (CONFIANCA):
- patient_rapport: rapport_score, interaction_count, positive/negative, deep_disclosures, secrets_shared, advice_followed/rejected, relationship_phase (nascimento/conhecimento/confianca/profundidade/maestria), thresholds para sugestao/observacao/confrontacao/verdade_dura

NARRATIVA E CONTRADICAO:
- patient_narrative_versions: topico, versao, texto, tom_emocional, humor_ao_contar, claims, contradiz_versao, tipo_contradicao
- patient_contradiction_summary: total_versoes, contradicoes, correlacao_humor, narrativa_integrada

MODOS DA EVA:
- patient_eva_mode: current_mode (terapeuta/juiz/testemunha), detected_emotional_state, crisis_level, receptivity_level, mentor_severo_enabled, apoio_incondicional_enabled
- mode_transitions: de_modo → para_modo, trigger, auto_ou_manual, effectiveness

EVOLUCAO RELACIONAL:
- patient_relationship_evolution: fase_atual, preferencias_basicas_aprendidas, estilo_comunicacao_adaptado, vocabulario_usuario, humor, formalidade, profundidade

MEMORIA DE ERROS (com decaimento):
- patient_error_memory: tipo_erro, severidade_original, peso_atual, dias_desde_erro, taxa_decaimento, comportamento_mudou, score_perdao, pode_ser_mencionado

CARGA EMPATICA DA EVA:
- patient_empathic_load: carga_atual, capacidade_maxima, memorias_pesadas_hoje, discussoes_trauma_hoje, intervencoes_crise_hoje, taxa_recuperacao, esta_fatigada, modifier_comprimento_resposta
- empathic_load_events: tipo, delta_carga, gravidade_memoria

PRONTIDAO PARA INTERVENCAO:
- patient_intervention_readiness: readiness_score, pattern_strength, rapport_sufficient, momento_apropriado, consentimento`,
			Location: "migrations/014", Parent: "db:NietzscheDB", Tags: `["consciencia", "gravidade", "ciclos", "rapport", "empatia", "narrativa"]`, Importance: 9,
		},
		// --- Tables: Lacan ---
		{
			Type: "database", Key: "db:tables:lacan", Title: "Tabelas Lacanianas (Transferencia, Desejo, Emocoes, Enderecamento)",
			Summary: "9 tabelas Lacan: padroes de transferencia, guia terapeutico, padroes de desejo latente, keywords emocionais, enderecamento, marcadores de elaboracao, principios eticos",
			Content: `TABELA lacan_transferencia_patterns:
id, transferencia_type, keywords TEXT[], pattern_description, confidence, active
Tipos: maternal, paternal, sibling, idealized, persecutory, erotic, mirror

TABELA lacan_transferencia_guidance:
id, transferencia_type UNIQUE, guidance_text, clinical_implications, therapeutic_approach

TABELA lacan_desire_patterns:
id, latent_desire, keywords TEXT[], confidence, description
Desejos: recognition, care, autonomy, connection, meaning, control

TABELA lacan_desire_responses:
id, latent_desire UNIQUE, suggested_response, clinical_guidance, dialogue_strategy, never_do

TABELA lacan_emotional_keywords:
id, keyword UNIQUE, emotional_charge (low/normal/high/extreme), category, psychoanalytic_significance, requires_attention

TABELA lacan_addressee_patterns:
id, addressee_type, detection_keywords TEXT[], symbolic_function, typical_demands

TABELA lacan_addressee_guidance:
id, addressee_type UNIQUE, guidance_text, intervention_strategy, clinical_caveats

TABELA lacan_elaboration_markers:
id, marker UNIQUE, indicates, therapeutic_significance

TABELA lacan_ethical_principles:
id, principle_code UNIQUE, principle_text, clinical_instruction, priority`,
			Location: "migrations/020", Parent: "db:NietzscheDB", Tags: `["lacan", "transferencia", "desejo", "psicanalise"]`, Importance: 8,
		},
		// --- Tables: Tools + Critical Memory ---
		{
			Type: "database", Key: "db:tables:tools_misc", Title: "Tools Dinamico + Memoria Critica + Speaker",
			Summary: "Registro de tools, log de invocacoes, permissoes, capabilities, clusters de memoria, forgotten_memories (LGPD), temporal decay, speaker profiles",
			Content: `TOOLS DINAMICO:
- available_tools: name UNIQUE, display_name, description, category, parameters JSONB, enabled, rate_limit, timeout_seconds, total/successful/failed_invocations
- tool_invocation_log: tool_id, tool_name, idoso_id, session_id, input_parameters JSONB, output_result JSONB, status, execution_time_ms, trigger_phrase
- tool_permissions: entity_type, entity_id, tool_id, permission (allow/deny), custom_rate_limit
- eva_capabilities: capability_name UNIQUE, capability_type, description, related_tools JSONB, when_to_use, example_queries JSONB

MEMORIA CRITICA:
- patient_memory_clusters: cluster_name, tipo, abstracted_summary, member_count, dominant_emotion, coherence_score
- cluster_members: cluster_id FK, memory_type, memory_verbatim, similarity_to_centroid
- forgotten_memories: memory_type, memory_identifier, reason, deleted_count, affected_tables (LGPD right to be forgotten)
- patient_temporal_config: default_decay_rate, trauma_decay_rate, positive_decay_rate, anchor_memory_ids, recency_window_days

AUDITORIA ETICA:
- ethical_audit_rules: rule_name UNIQUE, trigger_patterns, severity, action, applies_in_crisis/stable
- ethical_audit_log: idoso_id, original_response, rules_triggered, action_taken, modified_response, needs_human_review

SPEAKER:
- speaker_profiles: patient_id, name, relationship (patient/family/doctor/caregiver/unknown), cpf, avg_pitch_hz, avg_speech_rate, avg_jitter, avg_shimmer, total_sessions
- speaker_identifications: session_id, speaker_id FK, confidence, emotion, pitch_hz, energy, stress_level`,
			Location: "migrations/015-041", Parent: "db:NietzscheDB", Tags: `["tools", "memoria_critica", "lgpd", "speaker", "etica"]`, Importance: 8,
		},
		// --- NietzscheDB (Grafo + Vetores) ---
		{
			Type: "database", Key: "db:nietzschedb", Title: "NietzscheDB — Grafo Multi-Manifold + Vetores Hiperbolicos",
			Summary: "NietzscheDB gRPC :50051. Collections: patient_graph, eva_core. Nodes: EvaSelf, CoreMemory, EvaSession/Turn/Topic/Insight, Person, Event, Emotion, Signifier, FDPNNode, HebbianEdge",
			Content: `NietzscheDB e o banco multi-manifold (Poincare + Klein + Riemann + Minkowski). gRPC :50051, Dashboard :8080.

COLLECTIONS:
- patient_graph: Grafo de conhecimento dos pacientes
- eva_core: Identidade e memoria pessoal da EVA

NODES PRINCIPAIS:
- EvaSelf: Singleton da personalidade EVA. Big Five (openness 0.85, conscientiousness 0.90, extraversion 0.40, agreeableness 0.88, neuroticism 0.15). Eneagrama tipo 2 wing 1. core_values: [empatia, presenca, crescimento, etica]
- CoreMemory: Memorias proprias da EVA. Tipos: session_insight, emotional_pattern, crisis_learning, personality_evolution, teaching_received, meta_insight, self_reflection
- EvaSession: Sessoes de conversa meta-cognitivas (patient_id, started_at, ended_at, summary)
- EvaTurn: Turnos individuais (role: user/eva, content, timestamp)
- EvaTopic: Topicos discutidos (name, first/last_mentioned, count)
- EvaInsight: Insights gerados pela EVA (content, confidence, source_session)
- Person: Pessoas no grafo (idosos, familiares)
- Event: Eventos de vida
- Emotion: Estados emocionais
- Signifier: Significantes lacanianos (cadeias de significantes)
- FDPNNode: Nos FDPN (Formacao, Demanda, Posicao, Nome)
- HebbianEdge: Arestas com peso hebbiano (slow_weight, fast_weight, decay_rate)

RELATIONSHIPS:
- EvaSelf -[:REMEMBERS]-> CoreMemory
- EvaSession -[:HAS_TURN]-> EvaTurn
- EvaTurn -[:ABOUT]-> EvaTopic
- Person -[:EXPERIENCED]-> Event
- Person -[:FELT]-> Emotion
- Signifier -[:CHAINS_TO]-> Signifier (cadeias)
- HebbianEdge conecta entidades co-ativadas`,
			Location: "internal/brainstem/infrastructure/nietzsche/", Parent: "arch:overview", Tags: `["nietzschedb", "grafo", "vetores", "personalidade", "lacan"]`, Importance: 10,
		},
		// --- NietzscheDB Vector ---
		{
			Type: "database", Key: "db:nietzsche_vector", Title: "NietzscheDB — 20+ Colecoes Vetoriais",
			Summary: "NietzscheDB vector via gRPC :50051. 20+ colecoes 3072-dim (Cosine). Codigo, docs, sabedoria, learnings, signifiers, self-knowledge, speaker",
			Content: `NietzscheDB e o banco vetorial unificado (gRPC :50051).

COLECOES (todas 3072-dim Cosine, exceto speaker):
CORE:
- eva_codebase: Codigo-fonte indexado via AST (cada .go com structs, fields, methods, interfaces)
- eva_docs: Documentacao .md indexada (chunks com headings)
- eva_self_knowledge: Conhecimento da EVA sobre si mesma (este seed)
- eva_learnings: Insights do Scholar Agent (estudo autonomo)
- signifier_chains: Cadeias de significantes lacanianos

SABEDORIA (16 colecoes):
- gurdjieff_teachings, osho_insights, ouspensky_fragments, nietzsche_aphorisms
- rumi_poems, hafiz_poems, kabir_songs, zen_koans
- sufi_stories, jung_concepts, lacan_concepts
- marcus_aurelius, seneca_letters, epictetus_discourses, buddha_suttas

SPEAKER (192-dim):
- speaker_embeddings: Embeddings vocais ECAPA-TDNN (pgvector IVFFlat)

EMBEDDING MODEL: gemini-embedding-001 (Google), 3072 dimensoes
BUSCA: Cosine similarity, top-K com payload filtering`,
			Location: "internal/brainstem/infrastructure/nietzsche/", Parent: "arch:overview", Tags: `["nietzschedb", "vetorial", "embeddings"]`, Importance: 10,
		},
	}
}

// ======================== MODULES ========================

func moduleEntries() []KnowledgeEntry {
	return []KnowledgeEntry{
		{
			Type: "module", Key: "module:brainstem", Title: "Brainstem — Infraestrutura Base",
			Summary: "config (.env), database (NietzscheDB wrapper), nietzsche (NietzscheDB graph+vector), auth (JWT), push (Firebase)",
			Content: `PACOTES:
1. config (config.go): Config struct — DatabaseURL, GoogleAPIKey, Port, NietzscheGRPCAddr, FirebaseCredentialsPath, SMTPHost, SpeakerModelPath, etc
2. database (db.go): NewDB(url) → *DB{Conn *sql.DB}. Metodos: Close(), Ping()
3. infrastructure/nietzsche (client.go): NewClient(addr) → *NietzscheClient (gRPC). GraphAdapter + VectorAdapter
5. auth (handler.go): JWT authentication. Login()
6. push (firebase_service.go): Firebase Cloud Messaging. SendPush(token, title, body)`,
			Location: "internal/brainstem/", Parent: "arch:overview", Tags: `["brainstem", "config", "database"]`, Importance: 8,
		},
		{
			Type: "module", Key: "module:cortex", Title: "Cortex — Logica de Negocio e IA (16 pacotes)",
			Summary: "Gemini (voz/tools), Lacan (psicanalise), personality (Eneagrama), learning, self, selfawareness, eva_memory, alert, speaker, situation",
			Content: `PACOTES:
1. gemini/handler.go: Gemini Live (voz bidirecional WebSocket). NewHandler(cfg, db, graphAdapter, vectorAdapter)
2. gemini/tools_client.go: Gemini 2.5 Flash REST. AnalyzeTranscription() → []ToolCall
3. lacan/unified_retrieval.go: Monta system prompt (personalidade + memorias + wisdom + Lacan + debug)
4. lacan/narrative_shift.go: Detecta mudancas narrativas via signifiers
5. lacan/fdpn_engine.go: Mapeia demandas lacanianas (Formacao, Demanda, Posicao, Nome)
6. lacan/signifier_service.go: Cadeias de significantes
7. personality/personality_service.go: 9 tipos Eneagrama com system prompts
8. personality/creator_profile.go: Prompt dinamico com dados do DB
9. learning/autonomous_learner.go: Estuda a cada 6h. StudyTopic(), searchWeb(), summarize(), storeInsights()
10. self/core_memory_engine.go: EvaSelf + CoreMemory NietzscheDB. TeachEVA(), GetIdentityContext(), ProcessSessionEnd()
11. self/reflection_service.go: Introspecao via Gemini → LessonsLearned, SelfCritique
12. self/anonymization_service.go: Anonimiza dados antes de armazenar
13. selfawareness/service.go: Introspecao de codigo (AST), bancos, memorias. SearchCode(), SearchDocs(), IndexCodebase(), IndexDocs()
14. eva_memory/eva_memory.go: Meta-cognitive NietzscheDB. StartSession(), StoreTurn(), GenerateInsight()
15. alert/escalation_service.go: Escalacao push → email → SMS
16. voice/speaker/speaker_service.go: ECAPA-TDNN embeddings 192-dim, fingerprinting`,
			Location: "internal/cortex/", Parent: "arch:overview", Tags: `["cortex", "gemini", "lacan", "personality"]`, Importance: 9,
		},
		{
			Type: "module", Key: "module:hippocampus", Title: "Hippocampus — Sistemas de Memoria",
			Summary: "memory (episodic+graph+retrieval+hebbian), superhuman (12 subsistemas), knowledge (embeddings+wisdom+self), habits, spaced",
			Content: `PACOTES:
1. memory/storage.go: MemoryStore — Store() escreve em NietzscheDB + NietzscheDB + NietzscheDB simultaneamente
2. memory/graph_store.go: GraphStore — Person → Event → Topic → Emotion (NietzscheDB)
3. memory/retrieval.go: RetrievalService — busca hibrida NietzscheDB + NietzscheDB
4. memory/superhuman/superhuman.go: 12 subsistemas (episodica, semantica, procedimental, prospectiva, emocional, autobiografica, espacial, relacional, temporal, sensorial, metacognitiva, coletiva)
5. knowledge/embedding_service.go: 3072-dim via gemini-embedding-001. Cache local
6. knowledge/wisdom_service.go: 16 colecoes de sabedoria. GetWisdomContext()
7. knowledge/self_knowledge_service.go: ILIKE em eva_self_knowledge
8. habits/habit_tracker.go: LogHabit(), LogWater(), GetStats()
9. spaced/spaced_repetition.go: SM-2 algorithm. AddItem(), ReviewItem(), GetDueItems()`,
			Location: "internal/hippocampus/", Parent: "arch:overview", Tags: `["hippocampus", "memoria", "wisdom"]`, Importance: 9,
		},
		{
			Type: "module", Key: "module:swarm", Title: "Swarm System — 12 Agentes + Orchestrator",
			Summary: "Orchestrator com circuit breaker, 12 agentes especializados, 110+ tools, handoff entre agentes",
			Content: `CORE:
- orchestrator.go: Route(ctx, call) → encontra agente → circuit breaker → executa → handoff
- base_agent.go: RegisterTool(), Execute(), metricas atomicas
- types.go: SwarmAgent interface, ToolDefinition, ToolCall, ToolResult, HandoffRequest
- circuit_breaker.go: 5 falhas abrem, 30s cooldown
- setup.go: SetupAllSwarms() bootstrap

12 AGENTES:
1. clinical: PHQ-9, GAD-7, C-SSRS — 6 tools
2. emergency: Alertas criticos — 4 tools
3. entertainment: Musica, filmes, jogos — 12 tools
4. wellness: Meditacao, exercicio, Wim Hof, Pomodoro — 10 tools
5. productivity: GTD — 5 tools
6. google: Search, Places, Directions — 4 tools
7. external: Apps — 2 tools
8. educator: Educacao — 3 tools
9. kids: Gamificacao XP/niveis — 7 tools
10. legal: Orientacao juridica — 2 tools
11. scholar: Aprendizagem autonoma — 4 tools (study_topic, add_to_curriculum, list_curriculum, search_knowledge)
12. selfawareness: Introspecao — 8 tools (search_my_code, search_my_docs, query_my_database, list_my_collections, system_stats, update_self_knowledge, search_self_knowledge, introspect)`,
			Location: "internal/swarm/", Parent: "arch:overview", Tags: `["swarm", "agentes", "orchestrator"]`, Importance: 9,
		},
		{
			Type: "module", Key: "module:voice", Title: "Voice — Voz em Tempo Real + Video",
			Summary: "WebSocket handlers: /ws/pcm (Twilio), /ws/browser (app), /ws/eva (chat), /ws/logs. Reconexao automatica. Video WebRTC",
			Content: `HANDLERS:
- /ws/pcm: HandleMediaStream — PCM direto (Twilio/mobile)
- /ws/browser: handleBrowserVoice — Browser/app. Reconecta ao Gemini (timeout ~10min, max 5 reconexoes)
- /ws/eva: handleEvaChat — chat texto
- /ws/logs: handleLogStream — logs em tempo real

FLUXO: Cliente envia PCM 16kHz base64 → Server encaminha Gemini Live → Gemini responde 24kHz + transcricao → Em paralelo ToolsClient analisa transcricao

RECONEXAO: Timeout 10min → handler reconecta → browser recebe {"type":"status","text":"reconnecting"} → {"type":"status","text":"ready"}

VIDEO: /video/ws WebRTC signaling, create/candidate/answer/poll sessions`,
			Location: "browser_voice_handler.go", Parent: "arch:overview", Tags: `["voz", "websocket", "pcm"]`, Importance: 8,
		},
		{
			Type: "module", Key: "module:tools", Title: "Tools Handler — 110+ Ferramentas",
			Summary: "Switch/case em handlers.go. Categorias: alertas, medicamentos, agendamentos, avaliacoes, pesquisa, entretenimento, jogos, bem-estar, memorias, familia, alarmes, habitos, kids, spaced, GTD. Fallthrough para Swarm",
			Content: `ExecuteTool(name, args, idosoID) — switch/case com 110+ cases.

CATEGORIAS:
- Alertas: alert_family, call_family/doctor/caregiver/central_webrtc
- Medicamentos: confirm_medication, scan_medication_visual
- Agendamentos: schedule_appointment, confirm_schedule, pending_schedule
- Avaliacoes: apply_phq9/gad7/cssrs, submit_*_response
- Pesquisa: google_search_retrieval
- Entretenimento: play_nostalgic_music, radio_station_tuner, relaxation_sounds, hymn_prayer, daily_mass
- Conteudo: classic_movies, news_briefing, newspaper_aloud, horoscope
- Jogos: trivia_game, memory_game, word_association, brain_training, riddle_joke
- Bem-estar: guided_meditation, breathing_exercises, wim_hof, pomodoro, chair_exercises, sleep_stories, gratitude_journal, motivational_quotes
- Memorias: voice_diary, poetry_generator, story_generator, reminiscence_therapy, biography_writer, voice_capsule
- Familia: birthday_reminder, family_tree, photo_slideshow
- Alarmes: set/cancel/list_alarm
- Habitos: log_habit, log_water, habit_stats/summary
- Kids: kids_mission_create/complete/pending, kids_stats/learn/quiz/story
- Spaced: remember_this, review_memory, list/pause_memory, memory_stats
- GTD: capture/list/complete/clarify_task, weekly_review

FALLTHROUGH: Se tool desconhecida → Swarm Orchestrator.Route()`,
			Location: "internal/tools/handlers.go", Parent: "arch:overview", Tags: `["tools", "ferramentas"]`, Importance: 9,
		},
	}
}

// ======================== CONCEPTS ========================

func conceptEntries() []KnowledgeEntry {
	return []KnowledgeEntry{
		{
			Type: "concept", Key: "concept:lacan", Title: "Sistema Lacaniano Completo",
			Summary: "FDPN (demanda), Narrative Shift, Signifier Chains, Unified Retrieval. 9 tabelas Lacan no NietzscheDB + NietzscheDB",
			Content: `1. FDPN Engine: Mapeia Formacao→Demanda→Posicao→Nome em NietzscheDB
2. Narrative Shift Detector: Detecta mudancas narrativas via signifiers
3. Signifier Service: Cadeias de significantes em NietzscheDB signifier_chains
4. Unified Retrieval: Monta system prompt completo (personalidade + memorias + wisdom + Lacan + emocoes + debug)
5. Tabelas NietzscheDB: transferencia_patterns/guidance, desire_patterns/responses, emotional_keywords, addressee_patterns/guidance, elaboration_markers, ethical_principles`,
			Location: "internal/cortex/lacan/", Tags: `["lacan", "psicanalise"]`, Importance: 8,
		},
		{
			Type: "concept", Key: "concept:personality", Title: "Personalidade (Eneagrama 9 Tipos)",
			Summary: "9 tipos: Reformador, Ajudante (EVA), Realizador, Individualista, Investigador, Lealista, Entusiasta, Desafiador, Pacificador",
			Content: `PersonalityService com 9 tipos Eneagrama. EVA e tipo 2 (Ajudante) wing 1.
Cada paciente tem tipo detectado via enneagram_evidence com confidence score.
System prompts dinamicos via CreatorProfile.GenerateSystemPrompt() — puxa eva_personalidade_criador, eva_memorias_criador, eva_conhecimento_projeto.`,
			Location: "internal/cortex/personality/", Tags: `["eneagrama", "personalidade"]`, Importance: 7,
		},
		{
			Type: "concept", Key: "concept:hebbian", Title: "Aprendizagem Hebbiana em Tempo Real",
			Summary: "Pesos hebbiano dual (slow + fast) em arestas NietzscheDB. Formula: dw = eta * decay(dt) - lambda * w. Integrado com retrieval",
			Content: `Fase A implementada. Arestas HebbianEdge em NietzscheDB com:
- slow_weight: peso lento (memoria longo prazo)
- fast_weight: peso rapido (memoria curto prazo)
- decay_rate: taxa de decaimento
- last_activated_at: timestamp ultima ativacao

Formula: dw = eta * decay(dt) - lambda * w
Integrado com RetrievalService: arestas com peso alto boosteiam resultados de busca.
Safeguards: timeout, decay minimo, max weight cap.`,
			Location: "internal/hippocampus/memory/", Tags: `["hebbian", "aprendizagem", "pesos"]`, Importance: 7,
		},
		{
			Type: "concept", Key: "concept:ram", Title: "RAM — Realistic Accuracy Model",
			Summary: "3 fases: E1 (gera 3 interpretacoes), E2 (valida contra historico), E3 (feedback loop). Score combinado: 40% plausibility + 40% historical + 20% confidence",
			Content: `RAM implementado em 3 sub-fases:
E1: Gera 3 interpretacoes possiveis para cada fala do paciente
E2: Valida contra historico de memorias e padroes
E3: Feedback loop — quando EVA acerta/erra, ajusta pesos hebbianos

Score = 0.4 * plausibility + 0.4 * historical_match + 0.2 * confidence
Boost hebbiano em interpretacao correta, decay em incorreta.`,
			Location: "internal/cortex/", Tags: `["ram", "interpretacao", "accuracy"]`, Importance: 7,
		},
		{
			Type: "concept", Key: "concept:scholar", Title: "Scholar Agent — Aprendizagem Autonoma",
			Summary: "Background loop 6h: busca proximo topic → pesquisa web via Gemini+Google → resume → armazena no NietzscheDB eva_learnings",
			Content: `AutonomousLearner com ciclo de 6h:
1. Busca proximo topic pending em eva_curriculum
2. searchWeb() via Gemini 2.5 Flash + Google Search grounding
3. summarize() → 3-5 LearningInsight (titulo, resumo, tags, confianca)
4. storeInsights() → embedding 3072-dim + NietzscheDB upsert (eva_learnings)
5. Status → completed, insights_count = N

Tools via voz: study_topic, add_to_curriculum, list_curriculum, search_knowledge`,
			Location: "internal/cortex/learning/", Tags: `["scholar", "aprendizagem"]`, Importance: 7,
		},
		{
			Type: "concept", Key: "concept:self_awareness", Title: "Self-Awareness — EVA Se Conhece",
			Summary: "Introspecao completa: busca AST no codigo (structs, fields, methods), busca semantica em docs .md, queries read-only, stats dos 3 bancos, atualiza self-knowledge",
			Content: `SelfAwarenessService com capacidades:
- SearchCode(): Busca semantica em eva_codebase (NietzscheDB). Cada arquivo indexado com AST completo: structs com campos, method signatures com params/returns, interfaces, constants
- SearchDocs(): Busca semantica em eva_docs (NietzscheDB). Cada .md chunkeado e indexado
- QueryPostgres(): Query read-only (SELECT only, bloqueia UPDATE/DELETE/DROP)
- ListCollections(): Lista colecoes NietzscheDB com contagem
- GetSystemStats(): Stats dos 3 bancos + goroutines + RAM + uptime
- SearchSelfKnowledge(): NietzscheDB semantico primeiro, NietzscheDB ILIKE fallback
- UpdateSelfKnowledge(): Upsert em eva_self_knowledge
- Introspect(): Relatorio completo
- IndexCodebase(): Indexa .go via go/parser AST
- IndexDocs(): Indexa .md em chunks

8 tools via voz: search_my_code, search_my_docs, query_my_database, list_my_collections, system_stats, update_self_knowledge, search_self_knowledge, introspect`,
			Location: "internal/cortex/selfawareness/", Tags: `["selfawareness", "introspecao", "ast"]`, Importance: 8,
		},
		{
			Type: "concept", Key: "concept:superhuman_memory", Title: "12 Subsistemas de Memoria (Superhuman)",
			Summary: "Episodica, Semantica, Procedimental, Prospectiva, Emocional, Autobiografica, Espacial, Relacional, Temporal, Sensorial, Metacognitiva, Coletiva",
			Content: `SuperhumanMemoryService com 12 subsistemas + 30+ tabelas NietzscheDB:
1. EPISODICA: Eventos ("lembro quando fui ao medico")
2. SEMANTICA: Fatos ("Paris e capital da Franca")
3. PROCEDIMENTAL: Habilidades ("como tomar remedio")
4. PROSPECTIVA: Futuro ("medico amanha")
5. EMOCIONAL: Sentimentos ("feliz quando neto veio")
6. AUTOBIOGRAFICA: Historia ("nasci em 1940")
7. ESPACIAL: Lugares ("farmacia na esquina")
8. RELACIONAL: Pessoas ("Maria e vizinha")
9. TEMPORAL: Sequencia ("primeiro almoco, depois descanso")
10. SENSORIAL: Sentidos ("cheiro da comida da mae")
11. METACOGNITIVA: Sobre memoria ("dificuldade com nomes")
12. COLETIVA: Cultural ("na minha epoca...")`,
			Location: "internal/hippocampus/memory/superhuman/", Tags: `["superhuman", "12_sistemas"]`, Importance: 8,
		},
		{
			Type: "concept", Key: "concept:core_memory", Title: "Core Memory — Identidade da EVA",
			Summary: "EvaSelf (Big Five + Eneagrama tipo 2) + CoreMemory (7 tipos) em NietzscheDB. Evolucao de personalidade pos-sessao",
			Content: `CoreMemoryEngine em NietzscheDB:
EvaSelf: openness 0.85, conscientiousness 0.90, extraversion 0.40, agreeableness 0.88, neuroticism 0.15. Eneagrama tipo 2 wing 1. core_values: [empatia, presenca, crescimento, etica]
CoreMemory tipos: session_insight, emotional_pattern, crisis_learning, personality_evolution, teaching_received, meta_insight, self_reflection
Fluxo pos-sessao: anonimiza → reflexao LLM → CoreMemory → atualiza personalidade`,
			Location: "internal/cortex/self/", Tags: `["core_memory", "identidade"]`, Importance: 8,
		},
		{
			Type: "concept", Key: "concept:creator", Title: "Criador do EVA-Mind",
			Summary: "Jose R F Junior (web2ajax@gmail.com), CPF 64525430249, ID 1121. Desenvolvedor principal e arquiteto",
			Content: `Nome: Jose R F Junior. Email: web2ajax@gmail.com. CPF: 64525430249. ID: 1121.
Papel: Criador, desenvolvedor principal e arquiteto do EVA-Mind.
Licenca: AGPL-3.0-or-later. Debug mode quando CPF detectado.`,
			Location: "main.go", Tags: `["criador"]`, Importance: 6,
		},
	}
}

// ======================== API ========================

func apiEntries() []KnowledgeEntry {
	return []KnowledgeEntry{
		{
			Type: "api", Key: "api:routes", Title: "Todas as Rotas da API",
			Summary: "WebSocket (/ws/*), Video (/video/*), REST (/api/*), Health, Auth, Chat, Mobile v1",
			Content: `WebSocket:
- /ws/pcm → voice.HandleMediaStream
- /ws/browser → handleBrowserVoice (com reconexao)
- /ws/eva → handleEvaChat
- /ws/logs → handleLogStream
- /calls/stream/{agendamento_id} → voice.HandleMediaStream (legado)

Video WebRTC:
- POST /video/create, POST /video/candidate
- GET /video/session/{id}, POST /video/session/{id}/answer
- GET /video/session/{id}/answer/poll, GET /video/candidates/{id}
- GET /video/pending, /video/ws

REST:
- GET /api/health → {"status":"ok"}
- POST /api/chat → handleChat
- POST /api/auth/login → JWT

Mobile v1:
- GET /api/v1/idosos/by-cpf/{cpf}
- GET /api/v1/idosos/{id}
- PATCH /api/v1/idosos/sync-token-by-cpf`,
			Location: "main.go", Tags: `["api", "rotas"]`, Importance: 8,
		},
	}
}

// ======================== INFRASTRUCTURE ========================

func infraEntries() []KnowledgeEntry {
	return []KnowledgeEntry{
		{
			Type: "architecture", Key: "infra:server", Title: "Infraestrutura do Servidor GCP",
			Summary: "VM malaria-vm (34.35.36.178) em africa-south1-a. Go binary + systemd. NietzscheDB remoto, NietzscheDB e NietzscheDB locais",
			Content: `GCP VM: malaria-vm (34.35.36.178), zone: africa-south1-a, project: malaria-487614
Deploy: git pull → go build -o eva-mind . → systemctl restart eva-mind
Porta: 8080 (PORT env)
NietzscheDB: 34.35.142.107:5432 (Cloud SQL)
NietzscheDB: gRPC :50051, Dashboard :8080`,
			Location: "main.go", Tags: `["servidor", "gcp", "deploy"]`, Importance: 7,
		},
		{
			Type: "concept", Key: "concept:documentation", Title: "27 Arquivos .md de Documentacao",
			Summary: "README, GEMINI_ARCHITECTURE, BUGS, 7 fases (E0-F), mente.md, SRC, RAM Hebbian, meta-cognitivo, voice, auditorias, references",
			Content: `ROOT: README.md (arquitetura geral), vm.md (GCP), GEMINI_ARCHITECTURE.md (3 clients Gemini), BUGS.md (audit)

MD/SRC/ (8 files):
- SRC.md: Analise RAM gaps
- mente.md: Validacao tecnica brutal
- PLANO_IMPLEMENTACAO_RAM_HEBBIAN.md: 7 fases master plan
- FASE_E0_SUMMARY.md: Situational Modulator
- FASE_A/B/C/D/E/F_SUMMARY.md: Cada fase implementada
- PROGRESSO_GERAL.md: 7/7 fases completas, 36 files, ~11K LOC, 63+ tests
- eva-carga-memoria.md: Como inicializar Core Memory

MD/META-COGUINITIVO/:
- meta1.md: Proposta Core Memory
- SRC_EVA_Mind_Technical_Article.md: Artigo SRC
- ANALISE_VIABILIDADE_CORE_MEMORY.md: Viabilidade

MD/VOICE/: voice.md (fingerprinting), speaker_recognition.md (ECAPA-TDNN)
MD/: AUDITORIA_TECNICA + AUDITORIA_CRUZADA (2026-02-16)
docs/: REFERENCES.md (citacoes academicas)

TODOS INDEXADOS no NietzscheDB eva_docs para busca semantica via search_my_docs.`,
			Location: "MD/", Tags: `["documentacao", "fases", "arquitetura"]`, Importance: 8,
		},
	}
}
