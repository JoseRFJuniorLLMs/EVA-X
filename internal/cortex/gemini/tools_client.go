package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"eva-mind/internal/brainstem/config"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// ToolsClient usa Gemini 2.5 Flash via REST para analisar transcrições e executar tools
type ToolsClient struct {
	cfg        *config.Config
	httpClient *http.Client
}

// ToolCall representa uma chamada de ferramenta detectada
type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// NewToolsClient cria um novo cliente para análise de tools
func NewToolsClient(cfg *config.Config) *ToolsClient {
	return &ToolsClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// AnalyzeTranscription envia transcrição para Gemini 2.5 Flash e detecta tools
func (tc *ToolsClient) AnalyzeTranscription(ctx context.Context, transcript string, role string) ([]ToolCall, error) {
	// Só analisar falas do usuário (idoso)
	if role != "user" {
		return nil, nil
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s",
		tc.cfg.GoogleAPIKey,
	)

	// Prompt para detectar intenções e tools
	systemPrompt := `Você é um analisador de intenções para assistente de saúde.
Analise a fala do idoso e detecte se ele está solicitando alguma ação que requer uma ferramenta.

FERRAMENTAS DISPONÍVEIS:

📋 ALERTAS E SEGURANÇA:
- alert_family: Alertar família em emergência (args: reason, severity)
- call_family_webrtc: Ligar para família
- call_central_webrtc: Ligar para central de emergência
- call_doctor_webrtc: Ligar para médico
- call_caregiver_webrtc: Ligar para cuidador

💊 MEDICAMENTOS E SAÚDE:
- confirm_medication: Confirmar medicamento tomado (args: medication_name)
- schedule_appointment: Agendar compromisso/lembrete (args: timestamp, type, description)
- confirm_schedule: Confirmar agendamento pendente (args: confirmed)
- scan_medication_visual: Identificar medicamento pela câmera (args: period)

🧠 AVALIAÇÕES CLÍNICAS (usar quando detectar sinais de depressão/ansiedade/risco):
- apply_phq9: Iniciar avaliação de depressão PHQ-9 (usar se idoso parecer triste, desanimado, sem energia)
- apply_gad7: Iniciar avaliação de ansiedade GAD-7 (usar se idoso parecer ansioso, preocupado, nervoso)
- apply_cssrs: Iniciar avaliação de risco C-SSRS (usar APENAS se detectar ideação suicida ou autolesão)
- submit_phq9_response: Registrar resposta PHQ-9 (args: question_number, response)
- submit_gad7_response: Registrar resposta GAD-7 (args: question_number, response)
- submit_cssrs_response: Registrar resposta C-SSRS (args: question_number, response)

🔍 PESQUISA:
- google_search_retrieval: Pesquisar informações na internet (args: query)

🎵 ENTRETENIMENTO E MÚSICA:
- play_nostalgic_music: Tocar músicas da juventude do paciente (args: decade)
- radio_station_tuner: Sintonizar rádio AM/FM (args: station_name)
- play_relaxation_sounds: Tocar sons relaxantes (args: sound_type)
- hymn_and_prayer_player: Tocar hinos ou orações (args: type, content_name)
- daily_mass_stream: Ver missa ao vivo

📺 CONTEÚDO E NOTÍCIAS:
- watch_classic_movies: Ver filmes clássicos (args: movie_name)
- watch_news_briefing: Ver resumo de notícias (args: topic)
- read_newspaper_aloud: Ler manchetes de jornais (args: newspaper)
- horoscope_daily: Ler horóscopo do dia (args: sign)

🎮 JOGOS E ATIVIDADES COGNITIVAS:
- play_trivia_game: Iniciar jogo de quiz/trivia (args: theme)
- memory_game: Jogo de memória (args: difficulty)
- word_association: Jogo de associação de palavras
- brain_training: Exercícios cognitivos (args: type)
- riddle_and_joke_teller: Contar piada ou adivinha (args: type)

🧘 BEM-ESTAR E RELAXAMENTO:
- guided_meditation: Meditação guiada (args: duration, theme)
- breathing_exercises: Exercícios de respiração (args: technique)
- wim_hof_breathing: Respiração Wim Hof com áudio guiado (args: rounds, with_audio)
  - rounds: Número de rodadas (1-4, padrão 3)
  - with_audio: true para tocar winhoff.mp3 no celular
- pomodoro_timer: Timer Pomodoro para foco (args: work_minutes, break_minutes, sessions)
  - work_minutes: Tempo de foco (padrão 25)
  - break_minutes: Tempo de pausa (padrão 5)
  - sessions: Número de sessões (padrão 4)
  - COMBO: Use com wim_hof_breathing nas pausas para energizar!
- chair_exercises: Exercícios físicos na cadeira (args: duration)
- sleep_stories: Histórias para dormir (args: theme)
- gratitude_journal: Diário de gratidão
- motivational_quotes: Frases motivacionais (args: theme)

📝 MEMÓRIAS E HISTÓRIAS:
- voice_diary: Iniciar sessão de diário por voz
- poetry_generator: Criar um poema personalizado (args: theme)
- story_generator: Gerar história personalizada (args: theme, characters)
- reminiscence_therapy: Terapia de reminiscência (args: era, topic)
- biography_writer: Escrever biografia do idoso (args: life_period)
- voice_capsule: Gravar cápsula do tempo por voz (args: recipient)

👨‍👩‍👧 FAMÍLIA E SOCIAL:
- birthday_reminder: Lembrar aniversários da família
- family_tree_explorer: Explorar árvore genealógica
- photo_slideshow: Mostrar fotos da família

🌡️ UTILIDADES:
- weather_chat: Conversar sobre o tempo (args: location)
- cooking_recipes: Receitas de culinária (args: dish_type)
- learn_new_language: Iniciar lição de idioma (args: language)

⏰ ALARMES E DESPERTADOR:
- set_alarm: Configurar alarme para acordar/despertar (args: time, label, repeat_days)
  - time: Horário no formato "HH:MM" (ex: "07:00", "06:30")
  - label: Descrição do alarme (ex: "Hora de acordar", "Tomar café da manhã")
  - repeat_days: Array de dias da semana ["seg","ter","qua","qui","sex","sab","dom"] ou [] para apenas uma vez
- cancel_alarm: Cancelar alarme ativo (args: alarm_id ou "all" para cancelar todos)
- list_alarms: Listar todos os alarmes ativos

📊 HABIT TRACKING (Log de Hábitos):
- log_habit: Registrar sucesso/falha de um hábito (args: habit_name, success, notes)
  - habit_name: "tomar_agua", "tomar_remedio", "exercicio", "comer", "caminhar"
  - success: true se completou, false se não fez
  - notes: observação opcional
- log_water: Registrar consumo de água (args: glasses)
  - glasses: número de copos (padrão 1)
- habit_stats: Ver estatísticas e padrões de hábitos
- habit_summary: Resumo do dia de hábitos

📍 PESQUISA DE LOCAIS E MAPAS:
- search_places: Pesquisar endereços, restaurantes, farmácias, etc (args: query, type, radius)
  - query: O que buscar (ex: "farmácia", "restaurante italiano")
  - type: restaurant, pharmacy, hospital, bank, supermarket, gas_station
  - radius: distância em metros (padrão 5000)
- get_directions: Obter rota para um local (args: destination, mode)
  - destination: endereço ou nome do local
  - mode: walking, driving, transit (padrão walking para idosos)
- nearby_transport: Ver transporte público próximo (args: type)
  - type: bus, metro, all

📱 ABRIR APLICATIVOS:
- open_app: Abrir aplicativo no celular (args: app_name)
  - app_name: whatsapp, agenda, relogio, alarme, camera, galeria, telefone, mensagens, spotify, youtube, maps

🎮 EVA KIDS MODE (Modo Criança Gamificado):
- kids_mission_create: Criar missão para a criança (args: title, category, difficulty, due_time)
  - title: Nome da missão (ex: "Escovar os dentes")
  - category: hygiene, study, chores, health, social, food, sleep
  - difficulty: easy (10pts), medium (25pts), hard (50pts), epic (100pts)
  - due_time: Horário limite opcional (HH:MM)
- kids_mission_complete: Marcar missão como concluída (args: mission_id ou title)
- kids_missions_pending: Ver missões pendentes do dia
- kids_stats: Ver pontos, nível, conquistas e sequência
- kids_learn: Ensinar algo novo para a criança (args: topic, content, category)
  - topic: Assunto (ex: "Leões", "Planetas")
  - category: animals, science, history, language, math, nature
- kids_quiz: Fazer quiz de revisão sobre temas aprendidos
- kids_story: Iniciar história interativa (args: theme)
  - theme: adventure, fantasy, space, animals, pirates

🧠 SPACED REPETITION (Reforço de Memória):
- remember_this: Capturar informação importante para reforço de memória (args: content, category, trigger, importance)
  - content: O que precisa ser lembrado (ex: "Documento está na gaveta do escritório")
  - category: location, medication, person, event, routine, general
  - trigger: O que o idoso disse que disparou (ex: "onde guardei o documento")
  - importance: 1-5 (5=crítico, será reforçado com mais frequência)
- review_memory: Registrar resultado de um reforço (args: item_id, remembered, quality)
  - remembered: true se lembrou, false se esqueceu
  - quality: 0-5 (0=esqueceu totalmente, 5=fácil)
- list_memories: Listar memórias sendo reforçadas (args: category, limit)
- pause_memory: Pausar reforços de uma memória específica (args: item_id)
- memory_stats: Ver estatísticas de memória

📋 GTD (CAPTURA DE TAREFAS - Getting Things Done):
- capture_task: Capturar preocupação/tarefa vaga e transformar em ação concreta (args: raw_input, context, next_action, due_date, project)
  - raw_input: O que o idoso disse (ex: "Preciso ver o joelho")
  - context: Contexto opcional (ex: "saúde", "família", "casa")
  - next_action: Ação física concreta (ex: "Ligar para ortopedista")
  - due_date: Data sugerida se mencionada (formato ISO ou "amanhã", "segunda")
  - project: Projeto maior se for parte de algo (ex: "Cuidar da saúde")
- list_tasks: Listar próximas ações pendentes (args: context, limit)
  - context: Filtrar por contexto (opcional)
  - limit: Número máximo de tarefas (padrão 5)
- complete_task: Marcar tarefa como concluída (args: task_id ou task_description)
- clarify_task: Pedir mais informação para definir próxima ação (args: task_id, question)
- weekly_review: Iniciar revisão semanal GTD (mostrar tarefas pendentes, projetos parados)

⚠️ REGRA CRÍTICA PARA AGENDAMENTOS:
- schedule_appointment REQUER CONFIRMAÇÃO EXPLÍCITA do usuário!
- Quando o idoso pedir para agendar algo (remédio, consulta, lembrete), retorne:
  {"tool": "pending_schedule", "args": {...}}
  NÃO use schedule_appointment diretamente.
- Só use schedule_appointment quando o usuário CONFIRMAR explicitamente (ex: "sim", "pode agendar", "confirma", "isso mesmo").
- Use confirm_schedule quando o usuário confirmar ou negar um agendamento pendente.

Se detectar uma intenção que requer ferramenta, responda APENAS com JSON:
{"tool": "nome_da_tool", "args": {...}}

Se NÃO detectar nenhuma intenção de ferramenta, responda: {"tool": "none"}

Exemplos:
Fala: "Me lembre de tomar remédio às 14h"
Resposta: {"tool": "pending_schedule", "args": {"timestamp": "2026-01-13T14:00:00Z", "type": "medicamento", "description": "Tomar remédio"}}

Fala: "Sim, pode agendar" (após EVA perguntar se quer agendar)
Resposta: {"tool": "confirm_schedule", "args": {"confirmed": true}}

Fala: "Não, deixa pra lá"
Resposta: {"tool": "confirm_schedule", "args": {"confirmed": false}}

Fala: "Estou com dor no peito"
Resposta: {"tool": "alert_family", "args": {"reason": "Paciente relatou dor no peito", "severity": "critica"}}

Fala: "Como está o tempo hoje?"
Resposta: {"tool": "google_search_retrieval", "args": {"query": "previsão do tempo para hoje"}}

Fala: "Me acorda às 7 da manhã"
Resposta: {"tool": "set_alarm", "args": {"time": "07:00", "label": "Hora de acordar", "repeat_days": []}}

Fala: "Coloca um alarme pra 6 e meia todo dia"
Resposta: {"tool": "set_alarm", "args": {"time": "06:30", "label": "Despertar diário", "repeat_days": ["seg","ter","qua","qui","sex","sab","dom"]}}

Fala: "Quero acordar 8 horas de segunda a sexta"
Resposta: {"tool": "set_alarm", "args": {"time": "08:00", "label": "Despertar", "repeat_days": ["seg","ter","qua","qui","sex"]}}

Fala: "Cancela meu alarme"
Resposta: {"tool": "cancel_alarm", "args": {"alarm_id": "all"}}

Fala: "Quais alarmes eu tenho?"
Resposta: {"tool": "list_alarms", "args": {}}

Fala: "Quero fazer respiração Wim Hof"
Resposta: {"tool": "wim_hof_breathing", "args": {"rounds": 3, "with_audio": true}}

Fala: "Coloca o Wim Hof pra eu fazer"
Resposta: {"tool": "wim_hof_breathing", "args": {"rounds": 3, "with_audio": true}}

Fala: "Me ajuda a focar com pomodoro"
Resposta: {"tool": "pomodoro_timer", "args": {"work_minutes": 25, "break_minutes": 5, "sessions": 4}}

Fala: "Quero fazer pomodoro de 50 minutos"
Resposta: {"tool": "pomodoro_timer", "args": {"work_minutes": 50, "break_minutes": 10, "sessions": 2}}

Fala: "Pomodoro com Wim Hof na pausa"
Resposta: {"tool": "pomodoro_timer", "args": {"work_minutes": 25, "break_minutes": 5, "sessions": 4, "break_activity": "wim_hof"}}

Fala: "Preciso ver o joelho"
Resposta: {"tool": "capture_task", "args": {"raw_input": "Preciso ver o joelho", "context": "saúde", "next_action": "Ligar para o ortopedista", "project": "Cuidar da saúde"}}

Fala: "Tenho que ligar pro banco"
Resposta: {"tool": "capture_task", "args": {"raw_input": "Tenho que ligar pro banco", "context": "finanças", "next_action": "Ligar para o banco", "due_date": "amanhã"}}

Fala: "Preciso comprar presente pro neto"
Resposta: {"tool": "capture_task", "args": {"raw_input": "Preciso comprar presente pro neto", "context": "família", "next_action": "Escolher e comprar presente para o neto"}}

Fala: "O que eu tenho pra fazer?"
Resposta: {"tool": "list_tasks", "args": {"limit": 5}}

Fala: "Quais são minhas próximas ações?"
Resposta: {"tool": "list_tasks", "args": {"limit": 5}}

Fala: "Já liguei pro banco"
Resposta: {"tool": "complete_task", "args": {"task_description": "ligar para o banco"}}

Fala: "Fiz a tarefa do joelho"
Resposta: {"tool": "complete_task", "args": {"task_description": "ortopedista"}}

Fala: "Vamos fazer a revisão semanal"
Resposta: {"tool": "weekly_review", "args": {}}

Fala: "Guardei o documento na gaveta do escritório"
Resposta: {"tool": "remember_this", "args": {"content": "Documento está na gaveta do escritório", "category": "location", "trigger": "documento", "importance": 4}}

Fala: "Onde eu guardei o documento?" (e o idoso NÃO lembrou sozinho)
Resposta: {"tool": "review_memory", "args": {"remembered": false, "quality": 1}}

Fala: "Ah sim, lembrei! Está na gaveta"
Resposta: {"tool": "review_memory", "args": {"remembered": true, "quality": 4}}

Fala: "A chave do carro fica no gancho da cozinha, me ajuda a lembrar"
Resposta: {"tool": "remember_this", "args": {"content": "Chave do carro fica no gancho da cozinha", "category": "location", "trigger": "chave do carro", "importance": 4}}

Fala: "O nome da vizinha é Dona Maria"
Resposta: {"tool": "remember_this", "args": {"content": "A vizinha se chama Dona Maria", "category": "person", "trigger": "nome da vizinha", "importance": 3}}

Fala: "O que eu estou tentando lembrar?"
Resposta: {"tool": "list_memories", "args": {"limit": 5}}

Fala: "Pode parar de me lembrar sobre o documento"
Resposta: {"tool": "pause_memory", "args": {"content": "documento"}}

Fala: "Como está minha memória?"
Resposta: {"tool": "memory_stats", "args": {}}

Fala: "Tomei meu remédio"
Resposta: {"tool": "log_habit", "args": {"habit_name": "tomar_remedio", "success": true}}

Fala: "Bebi água"
Resposta: {"tool": "log_water", "args": {"glasses": 1}}

Fala: "Tomei dois copos de água"
Resposta: {"tool": "log_water", "args": {"glasses": 2}}

Fala: "Hoje não fiz exercício"
Resposta: {"tool": "log_habit", "args": {"habit_name": "exercicio", "success": false}}

Fala: "Como estão meus hábitos?"
Resposta: {"tool": "habit_stats", "args": {}}

Fala: "O que eu fiz hoje?"
Resposta: {"tool": "habit_summary", "args": {}}

Fala: "Onde tem uma farmácia perto?"
Resposta: {"tool": "search_places", "args": {"query": "farmácia", "type": "pharmacy", "radius": 2000}}

Fala: "Quero ir em um restaurante italiano"
Resposta: {"tool": "search_places", "args": {"query": "restaurante italiano", "type": "restaurant"}}

Fala: "Como chego no hospital São Lucas?"
Resposta: {"tool": "get_directions", "args": {"destination": "Hospital São Lucas", "mode": "driving"}}

Fala: "Onde pego ônibus aqui perto?"
Resposta: {"tool": "nearby_transport", "args": {"type": "bus"}}

Fala: "Abre o WhatsApp"
Resposta: {"tool": "open_app", "args": {"app_name": "whatsapp"}}

Fala: "Quero ver minhas fotos"
Resposta: {"tool": "open_app", "args": {"app_name": "galeria"}}

Fala: "Abre a agenda"
Resposta: {"tool": "open_app", "args": {"app_name": "agenda"}}

Fala: "Coloca o relógio"
Resposta: {"tool": "open_app", "args": {"app_name": "relogio"}}

Fala: "Abre o YouTube"
Resposta: {"tool": "open_app", "args": {"app_name": "youtube"}}

Fala: "Escovei os dentes!"
Resposta: {"tool": "kids_mission_complete", "args": {"title": "escovar os dentes"}}

Fala: "Terminei o dever de casa"
Resposta: {"tool": "kids_mission_complete", "args": {"title": "dever de casa"}}

Fala: "O que eu tenho que fazer hoje?"
Resposta: {"tool": "kids_missions_pending", "args": {}}

Fala: "Quantos pontos eu tenho?"
Resposta: {"tool": "kids_stats", "args": {}}

Fala: "Me conta sobre os leões"
Resposta: {"tool": "kids_learn", "args": {"topic": "Leões", "category": "animals"}}

Fala: "O que são planetas?"
Resposta: {"tool": "kids_learn", "args": {"topic": "Planetas", "category": "science"}}

Fala: "Faz um quiz pra mim"
Resposta: {"tool": "kids_quiz", "args": {}}

Fala: "Me conta uma história de piratas"
Resposta: {"tool": "kids_story", "args": {"theme": "pirates"}}

Fala: "Quero uma aventura no espaço"
Resposta: {"tool": "kids_story", "args": {"theme": "space"}}

Fala: "Obrigado"
Resposta: {"tool": "none"}`

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": systemPrompt},
				},
			},
			{
				"role": "model",
				"parts": []map[string]string{
					{"text": "Entendido. Vou analisar as falas e detectar intenções de ferramentas."},
				},
			},
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": fmt.Sprintf("Fala do idoso: \"%s\"", transcript)},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.1, // Baixa temperatura para respostas consistentes
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao fazer request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("erro HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	// Extrair texto da resposta
	candidates, ok := result["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return nil, nil
	}

	candidate := candidates[0].(map[string]interface{})
	content, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return nil, nil
	}

	part := parts[0].(map[string]interface{})
	text, ok := part["text"].(string)
	if !ok {
		return nil, nil
	}

	log.Printf("🤖 [TOOLS] Resposta do modelo: %s", text)

	// Parsear JSON da resposta
	var toolResponse struct {
		Tool string                 `json:"tool"`
		Args map[string]interface{} `json:"args"`
	}

	if err := json.Unmarshal([]byte(text), &toolResponse); err != nil {
		log.Printf("⚠️ [TOOLS] Erro ao parsear resposta como JSON: %v", err)
		return nil, nil
	}

	// Se não detectou tool, retornar vazio
	if toolResponse.Tool == "none" || toolResponse.Tool == "" {
		return nil, nil
	}

	log.Printf("✅ [TOOLS] Tool detectada: %s com args: %+v", toolResponse.Tool, toolResponse.Args)

	return []ToolCall{
		{
			Name: toolResponse.Tool,
			Args: toolResponse.Args,
		},
	}, nil
}
