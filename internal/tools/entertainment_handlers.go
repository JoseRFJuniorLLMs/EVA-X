package tools

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

// =============================================================================
// ENTERTAINMENT TOOLS HANDLERS
// =============================================================================
// 30 ferramentas de entretenimento para idosos
// Categorias: música, jogos, histórias, bem-estar, social, utilidades
// =============================================================================

// =============================================================================
// CATEGORIA 1: MÚSICA E ÁUDIO
// =============================================================================

// handlePlayNostalgicMusic toca músicas da época do paciente
func (h *ToolsHandler) handlePlayNostalgicMusic(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🎵 [MUSIC] Tocando música nostálgica para Idoso %d", idosoID)

	decade, _ := args["decade"].(string)
	artist, _ := args["artist"].(string)
	genre, _ := args["genre"].(string)
	mood, _ := args["mood"].(string)

	// Buscar preferências do paciente se não especificado
	if decade == "" || decade == "any" {
		decade = h.getPatientMusicPreference(idosoID, "decade")
	}

	// Selecionar música baseado nos critérios
	song := h.selectNostalgicSong(decade, artist, genre, mood)

	// Sinalizar mobile para tocar
	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "play_music", map[string]interface{}{
			"song_id":     song.ID,
			"song_title":  song.Title,
			"artist":      song.Artist,
			"decade":      decade,
			"stream_url":  song.StreamURL,
			"duration_ms": song.DurationMs,
		})
	}

	return map[string]interface{}{
		"status":  "playing",
		"song":    song.Title,
		"artist":  song.Artist,
		"message": fmt.Sprintf("Tocando '%s' de %s", song.Title, song.Artist),
	}, nil
}

// handlePlayRadioStation sintoniza estação de rádio
func (h *ToolsHandler) handlePlayRadioStation(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("📻 [RADIO] Sintonizando rádio para Idoso %d", idosoID)

	stationType, _ := args["station_type"].(string)
	stationName, _ := args["station_name"].(string)

	station := h.getRadioStation(stationType, stationName)

	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "play_radio", map[string]interface{}{
			"station_name": station.Name,
			"stream_url":   station.StreamURL,
			"type":         stationType,
		})
	}

	return map[string]interface{}{
		"status":  "playing",
		"station": station.Name,
		"message": fmt.Sprintf("Sintonizando %s", station.Name),
	}, nil
}

// handleNatureSounds reproduz sons da natureza
func (h *ToolsHandler) handleNatureSounds(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🌊 [NATURE] Reproduzindo sons da natureza para Idoso %d", idosoID)

	soundType, _ := args["sound_type"].(string)
	durationFloat, _ := args["duration_minutes"].(float64)
	volume, _ := args["volume"].(string)

	duration := int(durationFloat)
	if duration == 0 {
		duration = 30
	}
	if volume == "" {
		volume = "medium"
	}

	soundNames := map[string]string{
		"rain":        "Chuva suave",
		"ocean":       "Ondas do mar",
		"forest":      "Floresta tropical",
		"birds":       "Pássaros cantando",
		"fireplace":   "Lareira crepitando",
		"river":       "Rio correndo",
		"thunderstorm": "Tempestade distante",
		"wind":        "Vento suave",
	}

	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "play_nature_sound", map[string]interface{}{
			"sound_type":       soundType,
			"sound_name":       soundNames[soundType],
			"duration_minutes": duration,
			"volume":           volume,
		})
	}

	return map[string]interface{}{
		"status":   "playing",
		"sound":    soundNames[soundType],
		"duration": duration,
		"message":  fmt.Sprintf("Reproduzindo %s por %d minutos", soundNames[soundType], duration),
	}, nil
}

// handleReligiousContent reproduz conteúdo religioso
func (h *ToolsHandler) handleReligiousContent(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🙏 [RELIGIOUS] Conteúdo religioso para Idoso %d", idosoID)

	contentType, _ := args["content_type"].(string)
	religion, _ := args["religion"].(string)
	specificPrayer, _ := args["specific_prayer"].(string)

	if religion == "" {
		religion = h.getPatientReligionPreference(idosoID)
	}

	content := h.getReligiousContent(contentType, religion, specificPrayer)

	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "play_religious", map[string]interface{}{
			"content_type": contentType,
			"content_name": content.Name,
			"religion":     religion,
			"audio_url":    content.AudioURL,
			"text":         content.Text,
		})
	}

	return map[string]interface{}{
		"status":  "playing",
		"content": content.Name,
		"message": fmt.Sprintf("Reproduzindo %s", content.Name),
	}, nil
}

// =============================================================================
// CATEGORIA 2: JOGOS COGNITIVOS
// =============================================================================

// handlePlayTriviaGame inicia jogo de trivia
func (h *ToolsHandler) handlePlayTriviaGame(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🎯 [TRIVIA] Jogo de trivia para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	theme, _ := args["theme"].(string)
	difficulty, _ := args["difficulty"].(string)
	answer, _ := args["answer"].(string)

	switch action {
	case "start":
		question := h.getTriviaQuestion(theme, difficulty)
		return map[string]interface{}{
			"status":     "question",
			"question":   question.Text,
			"options":    question.Options,
			"theme":      theme,
			"difficulty": difficulty,
			"message":    question.Text,
		}, nil

	case "answer":
		correct, explanation := h.checkTriviaAnswer(idosoID, answer)
		if correct {
			return map[string]interface{}{
				"status":      "correct",
				"explanation": explanation,
				"message":     "Isso mesmo! Você acertou! " + explanation,
			}, nil
		}
		return map[string]interface{}{
			"status":      "incorrect",
			"explanation": explanation,
			"message":     "Não foi dessa vez. " + explanation,
		}, nil

	case "hint":
		hint := h.getTriviaHint(idosoID)
		return map[string]interface{}{
			"status":  "hint",
			"hint":    hint,
			"message": "Dica: " + hint,
		}, nil

	case "score":
		score := h.getTriviaScore(idosoID)
		return map[string]interface{}{
			"status":  "score",
			"correct": score.Correct,
			"total":   score.Total,
			"message": fmt.Sprintf("Você acertou %d de %d perguntas!", score.Correct, score.Total),
		}, nil

	case "end":
		h.endTriviaGame(idosoID)
		return map[string]interface{}{
			"status":  "ended",
			"message": "Jogo encerrado. Foi muito divertido jogar com você!",
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handleMemoryGame jogo de memória por voz
func (h *ToolsHandler) handleMemoryGame(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🧠 [MEMORY] Jogo de memória para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	gameType, _ := args["game_type"].(string)
	patientAnswer, _ := args["patient_answer"].(string)

	switch action {
	case "start":
		sequence := h.generateMemorySequence(gameType, 3) // Começa com 3 itens
		h.saveMemoryGameState(idosoID, sequence)
		return map[string]interface{}{
			"status":   "sequence",
			"sequence": sequence,
			"message":  fmt.Sprintf("Repita: %s", joinSequence(sequence)),
		}, nil

	case "check":
		correct, nextSequence := h.checkMemoryAnswer(idosoID, patientAnswer)
		if correct {
			h.saveMemoryGameState(idosoID, nextSequence)
			return map[string]interface{}{
				"status":       "correct",
				"next_sequence": nextSequence,
				"message":      fmt.Sprintf("Muito bem! Agora: %s", joinSequence(nextSequence)),
			}, nil
		}
		return map[string]interface{}{
			"status":  "incorrect",
			"message": "Ops! Não foi bem assim. Vamos tentar de novo?",
		}, nil

	case "score":
		score := h.getMemoryScore(idosoID)
		return map[string]interface{}{
			"status":        "score",
			"max_sequence":  score.MaxSequence,
			"message":       fmt.Sprintf("Sua maior sequência foi de %d itens!", score.MaxSequence),
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handleBrainTraining exercícios cognitivos
func (h *ToolsHandler) handleBrainTraining(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🧩 [BRAIN] Treino cerebral para Idoso %d", idosoID)

	exerciseType, _ := args["exercise_type"].(string)
	difficulty, _ := args["difficulty"].(string)
	action, _ := args["action"].(string)
	answer, _ := args["answer"].(string)

	switch action {
	case "start":
		exercise := h.generateBrainExercise(exerciseType, difficulty)
		h.saveBrainExerciseState(idosoID, exercise)
		return map[string]interface{}{
			"status":   "exercise",
			"type":     exerciseType,
			"question": exercise.Question,
			"message":  exercise.Question,
		}, nil

	case "answer":
		correct, explanation := h.checkBrainAnswer(idosoID, answer)
		nextExercise := h.generateBrainExercise(exerciseType, difficulty)
		result := "Correto!"
		if !correct {
			result = "Não foi dessa vez."
		}
		return map[string]interface{}{
			"status":        "answered",
			"correct":       correct,
			"explanation":   explanation,
			"next_question": nextExercise.Question,
			"message":       fmt.Sprintf("%s %s Próximo: %s", result, explanation, nextExercise.Question),
		}, nil

	case "hint":
		hint := h.getBrainHint(idosoID)
		return map[string]interface{}{
			"status":  "hint",
			"hint":    hint,
			"message": "Dica: " + hint,
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handleRiddlesAndJokes conta piadas e charadas
func (h *ToolsHandler) handleRiddlesAndJokes(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("😄 [HUMOR] Piadas e charadas para Idoso %d", idosoID)

	contentType, _ := args["content_type"].(string)
	theme, _ := args["theme"].(string)

	content := h.getHumorContent(contentType, theme)

	return map[string]interface{}{
		"status":  "content",
		"type":    contentType,
		"content": content.Text,
		"answer":  content.Answer, // Para charadas
		"message": content.Text,
	}, nil
}

// =============================================================================
// CATEGORIA 3: BEM-ESTAR
// =============================================================================

// handleGuidedMeditation conduz meditação guiada
func (h *ToolsHandler) handleGuidedMeditation(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🧘 [MEDITATION] Meditação guiada para Idoso %d", idosoID)

	technique, _ := args["technique"].(string)
	durationFloat, _ := args["duration_minutes"].(float64)
	backgroundSound, _ := args["background_sound"].(string)

	duration := int(durationFloat)
	if duration == 0 {
		duration = 10
	}

	meditation := h.getMeditationScript(technique, duration)

	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "start_meditation", map[string]interface{}{
			"technique":        technique,
			"duration_minutes": duration,
			"background_sound": backgroundSound,
			"script_id":        meditation.ID,
		})
	}

	return map[string]interface{}{
		"status":     "started",
		"technique":  technique,
		"duration":   duration,
		"intro":      meditation.Intro,
		"message":    meditation.Intro,
	}, nil
}

// handleBreathingExercises guia exercícios de respiração
func (h *ToolsHandler) handleBreathingExercises(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🌬️ [BREATHING] Exercício de respiração para Idoso %d", idosoID)

	technique, _ := args["technique"].(string)
	cyclesFloat, _ := args["cycles"].(float64)

	cycles := int(cyclesFloat)
	if cycles == 0 {
		cycles = 5
	}

	instructions := getBreathingInstructions(technique)

	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "start_breathing", map[string]interface{}{
			"technique":    technique,
			"cycles":       cycles,
			"instructions": instructions,
		})
	}

	return map[string]interface{}{
		"status":       "started",
		"technique":    technique,
		"cycles":       cycles,
		"instructions": instructions,
		"message":      fmt.Sprintf("Vamos fazer %d ciclos de respiração %s. %s", cycles, technique, instructions.Start),
	}, nil
}

// handleWimHofBreathing guia respiração Wim Hof com áudio opcional
func (h *ToolsHandler) handleWimHofBreathing(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("❄️ [WIM HOF] Respiração Wim Hof para Idoso %d", idosoID)

	roundsFloat, _ := args["rounds"].(float64)
	withAudio, _ := args["with_audio"].(bool)

	rounds := int(roundsFloat)
	if rounds == 0 || rounds > 4 {
		rounds = 3 // Padrão: 3 rodadas
	}

	// Instruções do Wim Hof
	instructions := WimHofInstructions{
		Intro:        "Vamos fazer respiração Wim Hof. Sente-se ou deite em lugar confortável. NUNCA faça isso na água ou dirigindo.",
		Breathing:    "Inspire profundamente pelo nariz, deixe o ar sair relaxadamente. Faça 30-40 respirações. Formigamento e tontura são normais.",
		Retention:    "Após a última expiração, segure com pulmões vazios pelo máximo que conseguir.",
		Recovery:     "Quando precisar respirar, inspire fundo e segure por 15 segundos.",
		BetweenRound: "Respire normalmente por um momento, depois repita.",
		Finish:       "Excelente! Você completou a respiração Wim Hof. Sinta a energia fluindo.",
	}

	// Notificar app para iniciar Wim Hof
	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "start_wim_hof", map[string]interface{}{
			"rounds":       rounds,
			"with_audio":   withAudio,
			"audio_file":   "winhoff.mp3",
			"instructions": instructions,
		})
	}

	audioMsg := ""
	if withAudio {
		audioMsg = " Vou tocar o áudio guiado para você."
	}

	return map[string]interface{}{
		"status":       "started",
		"rounds":       rounds,
		"with_audio":   withAudio,
		"audio_file":   "winhoff.mp3",
		"instructions": instructions,
		"message":      fmt.Sprintf("Vamos fazer %d rodadas de respiração Wim Hof.%s %s", rounds, audioMsg, instructions.Intro),
	}, nil
}

// WimHofInstructions estrutura de instruções do Wim Hof
type WimHofInstructions struct {
	Intro        string `json:"intro"`
	Breathing    string `json:"breathing"`
	Retention    string `json:"retention"`
	Recovery     string `json:"recovery"`
	BetweenRound string `json:"between_round"`
	Finish       string `json:"finish"`
}

// handlePomodoroTimer inicia timer Pomodoro
func (h *ToolsHandler) handlePomodoroTimer(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🍅 [POMODORO] Timer Pomodoro para Idoso %d", idosoID)

	workFloat, _ := args["work_minutes"].(float64)
	breakFloat, _ := args["break_minutes"].(float64)
	sessionsFloat, _ := args["sessions"].(float64)
	breakActivity, _ := args["break_activity"].(string)

	workMinutes := int(workFloat)
	breakMinutes := int(breakFloat)
	sessions := int(sessionsFloat)

	// Defaults Pomodoro clássico
	if workMinutes == 0 {
		workMinutes = 25
	}
	if breakMinutes == 0 {
		breakMinutes = 5
	}
	if sessions == 0 {
		sessions = 4
	}

	// Calcular tempo total
	totalMinutes := (workMinutes + breakMinutes) * sessions

	// Notificar app para iniciar Pomodoro
	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "start_pomodoro", map[string]interface{}{
			"work_minutes":   workMinutes,
			"break_minutes":  breakMinutes,
			"sessions":       sessions,
			"break_activity": breakActivity,
			"total_minutes":  totalMinutes,
		})
	}

	breakMsg := fmt.Sprintf("pausas de %d minutos", breakMinutes)
	if breakActivity == "wim_hof" {
		breakMsg = fmt.Sprintf("pausas de %d minutos com respiração Wim Hof", breakMinutes)
	}

	return map[string]interface{}{
		"status":         "started",
		"work_minutes":   workMinutes,
		"break_minutes":  breakMinutes,
		"sessions":       sessions,
		"break_activity": breakActivity,
		"total_minutes":  totalMinutes,
		"message":        fmt.Sprintf("Pomodoro iniciado! %d sessões de %d minutos de foco com %s. Tempo total: ~%d minutos.", sessions, workMinutes, breakMsg, totalMinutes),
	}, nil
}

// handleChairExercises guia exercícios na cadeira
func (h *ToolsHandler) handleChairExercises(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("💪 [EXERCISE] Exercícios na cadeira para Idoso %d", idosoID)

	bodyPart, _ := args["body_part"].(string)
	durationFloat, _ := args["duration_minutes"].(float64)
	intensity, _ := args["intensity"].(string)

	duration := int(durationFloat)
	if duration == 0 {
		duration = 10
	}
	if intensity == "" {
		intensity = "gentle"
	}

	exercises := h.getChairExercises(bodyPart, intensity, duration)

	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "start_exercises", map[string]interface{}{
			"body_part": bodyPart,
			"exercises": exercises,
			"duration":  duration,
		})
	}

	return map[string]interface{}{
		"status":         "started",
		"body_part":      bodyPart,
		"exercise_count": len(exercises),
		"first_exercise": exercises[0].Description,
		"message":        fmt.Sprintf("Vamos fazer exercícios para %s. Primeiro: %s", bodyPart, exercises[0].Description),
	}, nil
}

// handleGratitudeJournal diário de gratidão
func (h *ToolsHandler) handleGratitudeJournal(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🙏 [GRATITUDE] Diário de gratidão para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	gratitudeItems, _ := args["gratitude_items"].(string)

	switch action {
	case "add_entry":
		h.saveGratitudeEntry(idosoID, gratitudeItems)
		return map[string]interface{}{
			"status":  "saved",
			"message": "Que lindo! Anotei sua gratidão de hoje. É muito importante reconhecer as coisas boas da vida.",
		}, nil

	case "read_today":
		entries := h.getGratitudeEntries(idosoID, "today")
		return map[string]interface{}{
			"status":  "entries",
			"entries": entries,
			"message": fmt.Sprintf("Hoje você agradeceu por: %s", joinEntries(entries)),
		}, nil

	case "read_week":
		entries := h.getGratitudeEntries(idosoID, "week")
		return map[string]interface{}{
			"status":  "entries",
			"entries": entries,
			"count":   len(entries),
			"message": fmt.Sprintf("Essa semana você teve %d motivos para agradecer!", len(entries)),
		}, nil

	case "read_random":
		entry := h.getRandomGratitudeEntry(idosoID)
		return map[string]interface{}{
			"status":  "entry",
			"entry":   entry.Text,
			"date":    entry.Date,
			"message": fmt.Sprintf("Em %s você agradeceu por: %s", entry.Date, entry.Text),
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// =============================================================================
// CATEGORIA 4: SOCIAL E FAMÍLIA
// =============================================================================

// handleVoiceCapsule grava mensagens de voz para família
func (h *ToolsHandler) handleVoiceCapsule(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("💌 [CAPSULE] Cápsula de voz para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	recipient, _ := args["recipient"].(string)
	scheduledDate, _ := args["scheduled_date"].(string)
	occasion, _ := args["occasion"].(string)

	switch action {
	case "record":
		sessionID := fmt.Sprintf("capsule-%d-%d", idosoID, time.Now().Unix())
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "start_voice_recording", map[string]interface{}{
				"session_id": sessionID,
				"recipient":  recipient,
				"occasion":   occasion,
				"max_duration": 120, // 2 minutos máximo
			})
		}
		return map[string]interface{}{
			"status":     "recording",
			"session_id": sessionID,
			"message":    fmt.Sprintf("Gravando mensagem para %s. Pode falar!", recipient),
		}, nil

	case "send_now":
		err := h.sendVoiceCapsule(idosoID, recipient)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		return map[string]interface{}{
			"status":  "sent",
			"message": fmt.Sprintf("Mensagem enviada para %s!", recipient),
		}, nil

	case "schedule":
		h.scheduleVoiceCapsule(idosoID, recipient, scheduledDate, occasion)
		return map[string]interface{}{
			"status":  "scheduled",
			"date":    scheduledDate,
			"message": fmt.Sprintf("Mensagem agendada para %s será enviada em %s", recipient, scheduledDate),
		}, nil

	case "list":
		capsules := h.listVoiceCapsules(idosoID)
		return map[string]interface{}{
			"status":   "list",
			"capsules": capsules,
			"count":    len(capsules),
			"message":  fmt.Sprintf("Você tem %d mensagens gravadas", len(capsules)),
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handleBirthdayReminder gerencia lembretes de aniversário
func (h *ToolsHandler) handleBirthdayReminder(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🎂 [BIRTHDAY] Lembrete de aniversário para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	personName, _ := args["person_name"].(string)
	date, _ := args["date"].(string)

	switch action {
	case "check_today":
		birthdays := h.getTodayBirthdays(idosoID)
		if len(birthdays) == 0 {
			return map[string]interface{}{
				"status":  "none",
				"message": "Ninguém faz aniversário hoje.",
			}, nil
		}
		return map[string]interface{}{
			"status":    "found",
			"birthdays": birthdays,
			"message":   fmt.Sprintf("Hoje é aniversário de: %s!", joinNames(birthdays)),
		}, nil

	case "check_week":
		birthdays := h.getWeekBirthdays(idosoID)
		return map[string]interface{}{
			"status":    "found",
			"birthdays": birthdays,
			"count":     len(birthdays),
			"message":   fmt.Sprintf("%d aniversários essa semana", len(birthdays)),
		}, nil

	case "add":
		h.addBirthday(idosoID, personName, date)
		return map[string]interface{}{
			"status":  "added",
			"message": fmt.Sprintf("Anotei! Aniversário de %s em %s", personName, date),
		}, nil

	case "list_all":
		birthdays := h.getAllBirthdays(idosoID)
		return map[string]interface{}{
			"status":    "list",
			"birthdays": birthdays,
			"count":     len(birthdays),
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// =============================================================================
// CATEGORIA 5: HISTÓRIAS E NARRATIVAS
// =============================================================================

// handleStoryGenerator gera histórias personalizadas
func (h *ToolsHandler) handleStoryGenerator(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("📖 [STORY] Gerando história para Idoso %d", idosoID)

	storyType, _ := args["story_type"].(string)
	includeFamily, _ := args["include_family"].(bool)
	length, _ := args["length"].(string)
	setting, _ := args["setting"].(string)

	if length == "" {
		length = "medium"
	}

	// Buscar dados do paciente para personalização
	var familyNames []string
	if includeFamily {
		familyNames = h.getPatientFamilyNames(idosoID)
	}

	story := h.generateStory(storyType, length, setting, familyNames)

	return map[string]interface{}{
		"status":  "story",
		"title":   story.Title,
		"content": story.Content,
		"message": story.Content,
	}, nil
}

// handleReminiscenceTherapy terapia de reminiscência
func (h *ToolsHandler) handleReminiscenceTherapy(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("💭 [REMINISCENCE] Terapia de reminiscência para Idoso %d", idosoID)

	theme, _ := args["theme"].(string)
	useMusic, _ := args["use_music"].(bool)
	usePhotos, _ := args["use_photos"].(bool)

	// Buscar memórias relacionadas ao tema
	memories := h.getPatientMemories(idosoID, theme)

	// Gerar perguntas guia
	questions := getReminiscenceQuestions(theme)

	// Se usar música, buscar músicas da época
	var songSuggestion *Song
	if useMusic {
		songSuggestion = h.getSongForReminiscence(idosoID, theme)
	}

	return map[string]interface{}{
		"status":           "started",
		"theme":            theme,
		"opening_question": questions[0],
		"related_memories": memories,
		"song_suggestion":  songSuggestion,
		"use_photos":       usePhotos,
		"message":          questions[0],
	}, nil
}

// handleReadNewspaper lê notícias do dia
func (h *ToolsHandler) handleReadNewspaper(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("📰 [NEWS] Lendo notícias para Idoso %d", idosoID)

	category, _ := args["category"].(string)
	source, _ := args["source"].(string)
	detailLevel, _ := args["detail_level"].(string)

	if detailLevel == "" {
		detailLevel = "summary"
	}

	news := h.fetchNews(category, source, detailLevel)

	return map[string]interface{}{
		"status":   "news",
		"category": category,
		"articles": news,
		"count":    len(news),
		"message":  formatNewsForReading(news, detailLevel),
	}, nil
}

// handleDailyHoroscope lê horóscopo do dia
func (h *ToolsHandler) handleDailyHoroscope(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("⭐ [HOROSCOPE] Horóscopo para Idoso %d", idosoID)

	sign, _ := args["sign"].(string)

	// Se não especificou signo, buscar do perfil
	if sign == "" {
		sign = h.getPatientSign(idosoID)
	}

	horoscope := getHoroscope(sign)

	return map[string]interface{}{
		"status":    "horoscope",
		"sign":      sign,
		"sign_name": getSignName(sign),
		"message":   horoscope,
	}, nil
}

// =============================================================================
// CATEGORIA 6: UTILIDADES
// =============================================================================

// handleWeatherChat conversa sobre o tempo
func (h *ToolsHandler) handleWeatherChat(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🌤️ [WEATHER] Previsão do tempo para Idoso %d", idosoID)

	location, _ := args["location"].(string)
	forecastType, _ := args["forecast_type"].(string)

	if location == "" {
		location = h.getPatientLocation(idosoID)
	}
	if forecastType == "" {
		forecastType = "today"
	}

	weather := h.getWeatherForecast(location, forecastType)

	// Gerar dicas baseadas no tempo
	tips := generateWeatherTips(weather)

	return map[string]interface{}{
		"status":      "weather",
		"location":    location,
		"temperature": weather.Temperature,
		"condition":   weather.Condition,
		"tips":        tips,
		"message":     formatWeatherMessage(weather, tips),
	}, nil
}

// handleCookingRecipes compartilha receitas
func (h *ToolsHandler) handleCookingRecipes(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🍳 [RECIPES] Receitas para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	dishType, _ := args["dish_type"].(string)
	recipeName, _ := args["recipe_name"].(string)
	difficulty, _ := args["difficulty"].(string)

	switch action {
	case "search":
		recipes := h.searchRecipes(dishType, difficulty)
		return map[string]interface{}{
			"status":  "search_results",
			"recipes": recipes,
			"count":   len(recipes),
			"message": fmt.Sprintf("Encontrei %d receitas de %s", len(recipes), dishType),
		}, nil

	case "start_recipe":
		recipe := h.getRecipe(recipeName)
		h.saveRecipeState(idosoID, recipe, 0)
		return map[string]interface{}{
			"status":      "recipe_started",
			"recipe_name": recipe.Name,
			"ingredients": recipe.Ingredients,
			"first_step":  recipe.Steps[0],
			"message":     fmt.Sprintf("Vamos fazer %s! Ingredientes: %s. Primeiro passo: %s", recipe.Name, joinIngredients(recipe.Ingredients), recipe.Steps[0]),
		}, nil

	case "next_step":
		step, isLast := h.getNextRecipeStep(idosoID)
		if isLast {
			return map[string]interface{}{
				"status":  "recipe_complete",
				"message": "Pronto! Sua receita está finalizada. Bom apetite!",
			}, nil
		}
		return map[string]interface{}{
			"status":  "step",
			"step":    step,
			"message": step,
		}, nil

	case "repeat_step":
		step := h.getCurrentRecipeStep(idosoID)
		return map[string]interface{}{
			"status":  "step",
			"step":    step,
			"message": step,
		}, nil

	case "list_ingredients":
		ingredients := h.getRecipeIngredients(idosoID)
		return map[string]interface{}{
			"status":      "ingredients",
			"ingredients": ingredients,
			"message":     "Ingredientes: " + joinIngredients(ingredients),
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handleVoiceDiary diário de voz
func (h *ToolsHandler) handleVoiceDiary(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("📝 [DIARY] Diário de voz para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	date, _ := args["date"].(string)
	tag, _ := args["tag"].(string)

	switch action {
	case "record":
		sessionID := fmt.Sprintf("diary-%d-%d", idosoID, time.Now().Unix())
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "start_diary_recording", map[string]interface{}{
				"session_id":   sessionID,
				"tag":          tag,
				"max_duration": 300, // 5 minutos máximo
			})
		}
		return map[string]interface{}{
			"status":     "recording",
			"session_id": sessionID,
			"message":    "Gravando seu pensamento. Pode falar quando quiser...",
		}, nil

	case "play_today":
		entries := h.getDiaryEntries(idosoID, "today")
		return map[string]interface{}{
			"status":  "entries",
			"entries": entries,
			"count":   len(entries),
			"message": fmt.Sprintf("Você gravou %d pensamentos hoje", len(entries)),
		}, nil

	case "play_date":
		entries := h.getDiaryEntriesByDate(idosoID, date)
		return map[string]interface{}{
			"status":  "entries",
			"entries": entries,
			"date":    date,
		}, nil

	case "play_random":
		entry := h.getRandomDiaryEntry(idosoID)
		return map[string]interface{}{
			"status":  "entry",
			"entry":   entry,
			"message": fmt.Sprintf("Em %s você disse: %s", entry.Date, entry.Preview),
		}, nil

	case "list_recent":
		entries := h.getRecentDiaryEntries(idosoID, 10)
		return map[string]interface{}{
			"status":  "list",
			"entries": entries,
			"count":   len(entries),
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handleAudiobookReader lê audiobooks com controle de velocidade
func (h *ToolsHandler) handleAudiobookReader(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("📚 [AUDIOBOOK] Audiobook reader para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	bookTitle, _ := args["book_title"].(string)
	chapterFloat, _ := args["chapter"].(float64)
	speed, _ := args["speed"].(string)

	chapter := int(chapterFloat)
	if speed == "" {
		speed = "normal"
	}

	switch action {
	case "play":
		book := h.getAudiobook(bookTitle)
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "play_audiobook", map[string]interface{}{
				"book_id":    book.ID,
				"title":      book.Title,
				"chapter":    chapter,
				"speed":      speed,
				"stream_url": book.StreamURL,
			})
		}
		return map[string]interface{}{
			"status":  "playing",
			"book":    book.Title,
			"chapter": chapter,
			"message": fmt.Sprintf("Lendo '%s', capítulo %d", book.Title, chapter),
		}, nil

	case "pause":
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "pause_audiobook", nil)
		}
		return map[string]interface{}{"status": "paused", "message": "Audiobook pausado"}, nil

	case "resume":
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "resume_audiobook", nil)
		}
		return map[string]interface{}{"status": "resumed", "message": "Continuando..."}, nil

	case "list":
		books := h.listAvailableAudiobooks()
		return map[string]interface{}{"status": "list", "books": books}, nil

	case "search":
		books := h.searchAudiobooks(bookTitle)
		return map[string]interface{}{"status": "search", "results": books}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handlePodcastPlayer reproduz podcasts por categoria
func (h *ToolsHandler) handlePodcastPlayer(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🎙️ [PODCAST] Podcast player para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	category, _ := args["category"].(string)

	switch action {
	case "play":
		podcast := h.getPodcastByCategory(category)
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "play_podcast", map[string]interface{}{
				"podcast_id":   podcast.ID,
				"title":        podcast.Title,
				"episode":      podcast.Episode,
				"stream_url":   podcast.StreamURL,
			})
		}
		return map[string]interface{}{
			"status":  "playing",
			"podcast": podcast.Title,
			"episode": podcast.Episode,
			"message": fmt.Sprintf("Tocando '%s'", podcast.Title),
		}, nil

	case "pause":
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "pause_podcast", nil)
		}
		return map[string]interface{}{"status": "paused"}, nil

	case "resume":
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "resume_podcast", nil)
		}
		return map[string]interface{}{"status": "resumed"}, nil

	case "list":
		podcasts := h.listPodcastsByCategory(category)
		return map[string]interface{}{"status": "list", "podcasts": podcasts}, nil

	case "search":
		podcasts := h.searchPodcasts(category)
		return map[string]interface{}{"status": "search", "results": podcasts}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handleWordAssociation jogo de associação de palavras
func (h *ToolsHandler) handleWordAssociation(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🔤 [WORD] Associação de palavras para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	category, _ := args["category"].(string)
	response, _ := args["response"].(string)

	switch action {
	case "start":
		word := h.getWordForAssociation(category)
		h.saveWordGameState(idosoID, word)
		return map[string]interface{}{
			"status":  "started",
			"word":    word,
			"message": fmt.Sprintf("Qual palavra vem à sua mente quando você pensa em '%s'?", word),
		}, nil

	case "respond":
		isValid, feedback, nextWord := h.processWordResponse(idosoID, response)
		return map[string]interface{}{
			"status":    "response",
			"valid":     isValid,
			"feedback":  feedback,
			"next_word": nextWord,
			"message":   fmt.Sprintf("%s Próxima: qual palavra vem à sua mente com '%s'?", feedback, nextWord),
		}, nil

	case "end":
		score := h.getWordAssociationScore(idosoID)
		return map[string]interface{}{
			"status":  "ended",
			"score":   score,
			"message": fmt.Sprintf("Você fez %d associações. Ótimo exercício mental!", score),
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handleCompleteTheLyrics jogo de completar letra de música
func (h *ToolsHandler) handleCompleteTheLyrics(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🎤 [LYRICS] Complete a letra para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	decade, _ := args["decade"].(string)
	answer, _ := args["answer"].(string)

	switch action {
	case "start":
		lyric := h.getLyricChallenge(decade)
		h.saveLyricGameState(idosoID, lyric)
		return map[string]interface{}{
			"status":   "started",
			"song":     lyric.SongTitle,
			"artist":   lyric.Artist,
			"lyric":    lyric.PartialLyric,
			"message":  fmt.Sprintf("Complete a letra de '%s': %s...", lyric.SongTitle, lyric.PartialLyric),
		}, nil

	case "answer":
		correct, explanation := h.checkLyricAnswer(idosoID, answer)
		return map[string]interface{}{
			"status":      "answered",
			"correct":     correct,
			"explanation": explanation,
		}, nil

	case "skip":
		answer, nextLyric := h.skipLyricChallenge(idosoID)
		return map[string]interface{}{
			"status":      "skipped",
			"answer":      answer,
			"next_lyric":  nextLyric,
		}, nil

	case "hint":
		hint := h.getLyricHint(idosoID)
		return map[string]interface{}{"status": "hint", "hint": hint}, nil

	case "score":
		score := h.getLyricScore(idosoID)
		return map[string]interface{}{"status": "score", "correct": score.Correct, "total": score.Total}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handleSleepStories histórias calmas para induzir sono
func (h *ToolsHandler) handleSleepStories(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🌙 [SLEEP] História para dormir para Idoso %d", idosoID)

	storyTheme, _ := args["story_theme"].(string)
	includeBreathing, _ := args["include_breathing"].(bool)

	if storyTheme == "" {
		storyTheme = "nature"
	}

	story := h.getSleepStory(storyTheme, includeBreathing)

	if h.NotifyFunc != nil {
		h.NotifyFunc(idosoID, "play_sleep_story", map[string]interface{}{
			"story_id":          story.ID,
			"title":             story.Title,
			"audio_url":         story.AudioURL,
			"include_breathing": includeBreathing,
			"duration_minutes":  story.DurationMinutes,
		})
	}

	return map[string]interface{}{
		"status":   "playing",
		"story":    story.Title,
		"duration": story.DurationMinutes,
		"message":  fmt.Sprintf("Vou contar a história '%s'. Relaxe e feche os olhos...", story.Title),
	}, nil
}

// handleMotivationalQuotes citações de grandes pensadores
func (h *ToolsHandler) handleMotivationalQuotes(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("💡 [QUOTES] Frase motivacional para Idoso %d", idosoID)

	theme, _ := args["theme"].(string)
	authorType, _ := args["author_type"].(string)

	if theme == "" {
		theme = "general"
	}
	if authorType == "" {
		authorType = "any"
	}

	quote := h.getMotivationalQuote(theme, authorType)

	return map[string]interface{}{
		"status":  "quote",
		"quote":   quote.Text,
		"author":  quote.Author,
		"message": fmt.Sprintf("\"%s\" — %s", quote.Text, quote.Author),
	}, nil
}

// handleFamilyTreeExplorer navega pela genealogia
func (h *ToolsHandler) handleFamilyTreeExplorer(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("🌳 [FAMILY] Árvore genealógica para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	personName, _ := args["person_name"].(string)
	relation, _ := args["relation"].(string)
	story, _ := args["story"].(string)

	switch action {
	case "explore":
		tree := h.getFamilyTree(idosoID)
		return map[string]interface{}{
			"status":  "tree",
			"members": tree.Members,
			"message": fmt.Sprintf("Sua família tem %d pessoas registradas", len(tree.Members)),
		}, nil

	case "add_person":
		h.addFamilyMember(idosoID, personName, relation)
		return map[string]interface{}{
			"status":  "added",
			"person":  personName,
			"message": fmt.Sprintf("Adicionei %s como %s", personName, relation),
		}, nil

	case "add_story":
		h.addFamilyStory(idosoID, personName, story)
		return map[string]interface{}{
			"status":  "story_added",
			"person":  personName,
			"message": fmt.Sprintf("História sobre %s salva!", personName),
		}, nil

	case "view_tree":
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "show_family_tree", nil)
		}
		return map[string]interface{}{"status": "viewing"}, nil

	case "find_relation":
		relationInfo := h.findFamilyRelation(idosoID, personName)
		return map[string]interface{}{
			"status":   "relation",
			"person":   personName,
			"relation": relationInfo,
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handlePhotoSlideshow apresentação de fotos com narração
func (h *ToolsHandler) handlePhotoSlideshow(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("📷 [PHOTOS] Slideshow de fotos para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	album, _ := args["album"].(string)
	withMusic, _ := args["with_music"].(bool)
	comment, _ := args["comment"].(string)

	switch action {
	case "start":
		photos := h.getPhotoAlbum(idosoID, album)
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "start_slideshow", map[string]interface{}{
				"photos":     photos,
				"album":      album,
				"with_music": withMusic,
			})
		}
		return map[string]interface{}{
			"status":  "started",
			"album":   album,
			"count":   len(photos),
			"message": fmt.Sprintf("Mostrando álbum '%s' com %d fotos", album, len(photos)),
		}, nil

	case "pause":
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "pause_slideshow", nil)
		}
		return map[string]interface{}{"status": "paused"}, nil

	case "next":
		photo := h.getNextPhoto(idosoID)
		return map[string]interface{}{
			"status":      "next",
			"photo":       photo.URL,
			"description": photo.Description,
			"date":        photo.Date,
		}, nil

	case "previous":
		photo := h.getPreviousPhoto(idosoID)
		return map[string]interface{}{
			"status":      "previous",
			"photo":       photo.URL,
			"description": photo.Description,
		}, nil

	case "stop":
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "stop_slideshow", nil)
		}
		return map[string]interface{}{"status": "stopped"}, nil

	case "comment":
		h.savePhotoComment(idosoID, comment)
		return map[string]interface{}{
			"status":  "commented",
			"message": "Comentário salvo na foto!",
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// handleBiographyWriter ajuda a construir biografia como legado
func (h *ToolsHandler) handleBiographyWriter(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("📖 [BIOGRAPHY] Escritor de biografia para Idoso %d", idosoID)

	action, _ := args["action"].(string)
	lifeChapter, _ := args["life_chapter"].(string)

	switch action {
	case "start_session":
		questions := h.getBiographyQuestions(lifeChapter)
		h.saveBiographySession(idosoID, lifeChapter)
		return map[string]interface{}{
			"status":    "session_started",
			"chapter":   lifeChapter,
			"questions": questions,
			"message":   fmt.Sprintf("Vamos falar sobre '%s'. %s", lifeChapter, questions[0]),
		}, nil

	case "continue":
		nextQuestion := h.getNextBiographyQuestion(idosoID)
		return map[string]interface{}{
			"status":   "continuing",
			"question": nextQuestion,
		}, nil

	case "read_back":
		biography := h.getCurrentBiography(idosoID)
		return map[string]interface{}{
			"status":    "reading",
			"biography": biography,
			"message":   "Aqui está o que você já me contou...",
		}, nil

	case "export":
		exportURL := h.exportBiography(idosoID)
		return map[string]interface{}{
			"status":     "exported",
			"export_url": exportURL,
			"message":    "Sua biografia foi exportada e pode ser compartilhada com a família!",
		}, nil

	case "add_photo":
		if h.NotifyFunc != nil {
			h.NotifyFunc(idosoID, "select_photo_for_biography", nil)
		}
		return map[string]interface{}{
			"status":  "waiting_photo",
			"message": "Selecione uma foto para adicionar à biografia",
		}, nil
	}

	return map[string]interface{}{"error": "ação desconhecida"}, nil
}

// =============================================================================
// ESTRUTURAS DE SUPORTE
// =============================================================================

type Song struct {
	ID         string
	Title      string
	Artist     string
	StreamURL  string
	DurationMs int
}

type RadioStation struct {
	Name      string
	StreamURL string
}

type ReligiousContent struct {
	Name     string
	AudioURL string
	Text     string
}

type TriviaQuestion struct {
	Text    string
	Options []string
	Answer  string
	Hint    string
}

type TriviaScore struct {
	Correct int
	Total   int
}

type MemoryScore struct {
	MaxSequence int
}

type BrainExercise struct {
	Question string
	Answer   string
	Hint     string
}

type HumorContent struct {
	Text   string
	Answer string // Para charadas
}

type MeditationScript struct {
	ID    string
	Intro string
}

type BreathingInstructions struct {
	Start       string
	InhaleTime  int
	HoldTime    int
	ExhaleTime  int
}

type ChairExercise struct {
	Description string
	Duration    int
}

type GratitudeEntry struct {
	Text string
	Date string
}

type VoiceCapsule struct {
	ID        string
	Recipient string
	Date      string
	Occasion  string
}

type Birthday struct {
	Name string
	Date string
}

type Story struct {
	Title   string
	Content string
}

type NewsArticle struct {
	Title   string
	Summary string
	Source  string
}

type Weather struct {
	Temperature int
	Condition   string
	Humidity    int
}

type Recipe struct {
	Name        string
	Ingredients []string
	Steps       []string
}

type DiaryEntry struct {
	ID       string
	Date     string
	Preview  string
	AudioURL string
	Tag      string
}

type Audiobook struct {
	ID        string
	Title     string
	Author    string
	StreamURL string
}

type Podcast struct {
	ID        string
	Title     string
	Episode   string
	StreamURL string
}

type LyricChallenge struct {
	SongTitle    string
	Artist       string
	PartialLyric string
	FullLyric    string
}

type SleepStory struct {
	ID              string
	Title           string
	AudioURL        string
	DurationMinutes int
}

type MotivationalQuote struct {
	Text   string
	Author string
}

type FamilyTree struct {
	Members []FamilyMember
}

type FamilyMember struct {
	Name     string
	Relation string
}

type Photo struct {
	URL         string
	Description string
	Date        string
}

// =============================================================================
// FUNÇÕES AUXILIARES (stubs - implementar conforme necessário)
// =============================================================================

func (h *ToolsHandler) getPatientMusicPreference(idosoID int64, pref string) string {
	return "1970s"
}

func (h *ToolsHandler) selectNostalgicSong(decade, artist, genre, mood string) *Song {
	return &Song{
		ID:        "song-001",
		Title:     "Carinhoso",
		Artist:    "Pixinguinha",
		StreamURL: "https://stream.example.com/carinhoso.mp3",
		DurationMs: 180000,
	}
}

func (h *ToolsHandler) getRadioStation(stationType, stationName string) *RadioStation {
	return &RadioStation{
		Name:      "CBN",
		StreamURL: "https://stream.cbn.com.br/live",
	}
}

func (h *ToolsHandler) getPatientReligionPreference(idosoID int64) string {
	return "catholic"
}

func (h *ToolsHandler) getReligiousContent(contentType, religion, specific string) *ReligiousContent {
	return &ReligiousContent{
		Name:     "Pai Nosso",
		AudioURL: "https://audio.example.com/pai-nosso.mp3",
		Text:     "Pai nosso que estais no céu...",
	}
}

func (h *ToolsHandler) getTriviaQuestion(theme, difficulty string) *TriviaQuestion {
	return &TriviaQuestion{
		Text:    "Quem foi o primeiro presidente do Brasil?",
		Options: []string{"Dom Pedro I", "Deodoro da Fonseca", "Getúlio Vargas", "Juscelino Kubitschek"},
		Answer:  "Deodoro da Fonseca",
		Hint:    "Foi um militar",
	}
}

func (h *ToolsHandler) checkTriviaAnswer(idosoID int64, answer string) (bool, string) {
	return true, "Marechal Deodoro da Fonseca foi o primeiro presidente, em 1889."
}

func (h *ToolsHandler) getTriviaHint(idosoID int64) string {
	return "Foi um militar"
}

func (h *ToolsHandler) getTriviaScore(idosoID int64) *TriviaScore {
	return &TriviaScore{Correct: 5, Total: 7}
}

func (h *ToolsHandler) endTriviaGame(idosoID int64) {}

func (h *ToolsHandler) generateMemorySequence(gameType string, length int) []string {
	return []string{"3", "7", "2"}
}

func (h *ToolsHandler) saveMemoryGameState(idosoID int64, sequence []string) {}

func (h *ToolsHandler) checkMemoryAnswer(idosoID int64, answer string) (bool, []string) {
	return true, []string{"3", "7", "2", "9"}
}

func (h *ToolsHandler) getMemoryScore(idosoID int64) *MemoryScore {
	return &MemoryScore{MaxSequence: 6}
}

func (h *ToolsHandler) generateBrainExercise(exerciseType, difficulty string) *BrainExercise {
	return &BrainExercise{
		Question: "Quanto é 15 + 27?",
		Answer:   "42",
		Hint:     "Some primeiro as unidades",
	}
}

func (h *ToolsHandler) saveBrainExerciseState(idosoID int64, exercise *BrainExercise) {}

func (h *ToolsHandler) checkBrainAnswer(idosoID int64, answer string) (bool, string) {
	return true, "15 + 27 = 42"
}

func (h *ToolsHandler) getBrainHint(idosoID int64) string {
	return "Some primeiro as unidades"
}

func (h *ToolsHandler) getHumorContent(contentType, theme string) *HumorContent {
	return &HumorContent{
		Text:   "Por que o livro de matemática ficou triste? Porque tinha muitos problemas!",
		Answer: "",
	}
}

func (h *ToolsHandler) getMeditationScript(technique string, duration int) *MeditationScript {
	return &MeditationScript{
		ID:    "med-001",
		Intro: "Encontre uma posição confortável. Feche os olhos suavemente...",
	}
}

func getBreathingInstructions(technique string) *BreathingInstructions {
	switch technique {
	case "4-7-8":
		return &BreathingInstructions{
			Start:      "Inspire pelo nariz contando até 4...",
			InhaleTime: 4,
			HoldTime:   7,
			ExhaleTime: 8,
		}
	case "box_breathing":
		return &BreathingInstructions{
			Start:      "Inspire contando até 4...",
			InhaleTime: 4,
			HoldTime:   4,
			ExhaleTime: 4,
		}
	default:
		return &BreathingInstructions{
			Start:      "Respire profundamente...",
			InhaleTime: 4,
			HoldTime:   2,
			ExhaleTime: 4,
		}
	}
}

func (h *ToolsHandler) getChairExercises(bodyPart, intensity string, duration int) []*ChairExercise {
	return []*ChairExercise{
		{Description: "Gire a cabeça lentamente para a direita, depois para a esquerda", Duration: 30},
		{Description: "Levante os ombros em direção às orelhas e solte", Duration: 20},
	}
}

func (h *ToolsHandler) saveGratitudeEntry(idosoID int64, items string) {}

func (h *ToolsHandler) getGratitudeEntries(idosoID int64, period string) []string {
	return []string{"minha família", "o sol de hoje", "uma boa noite de sono"}
}

func (h *ToolsHandler) getRandomGratitudeEntry(idosoID int64) *GratitudeEntry {
	return &GratitudeEntry{Text: "minha família", Date: "15/01/2026"}
}

func (h *ToolsHandler) sendVoiceCapsule(idosoID int64, recipient string) error {
	return nil
}

func (h *ToolsHandler) scheduleVoiceCapsule(idosoID int64, recipient, date, occasion string) {}

func (h *ToolsHandler) listVoiceCapsules(idosoID int64) []*VoiceCapsule {
	return []*VoiceCapsule{}
}

func (h *ToolsHandler) getTodayBirthdays(idosoID int64) []*Birthday {
	return []*Birthday{}
}

func (h *ToolsHandler) getWeekBirthdays(idosoID int64) []*Birthday {
	return []*Birthday{}
}

func (h *ToolsHandler) addBirthday(idosoID int64, name, date string) {}

func (h *ToolsHandler) getAllBirthdays(idosoID int64) []*Birthday {
	return []*Birthday{}
}

func (h *ToolsHandler) getPatientFamilyNames(idosoID int64) []string {
	return []string{"Maria", "João", "Ana"}
}

func (h *ToolsHandler) generateStory(storyType, length, setting string, familyNames []string) *Story {
	return &Story{
		Title:   "A Aventura de Maria",
		Content: "Era uma vez...",
	}
}

func (h *ToolsHandler) getPatientMemories(idosoID int64, theme string) []string {
	return []string{}
}

func getReminiscenceQuestions(theme string) []string {
	questions := map[string][]string{
		"childhood": {"Onde você cresceu?", "Como era sua casa?", "Quem eram seus amigos?"},
		"youth":     {"O que você gostava de fazer quando jovem?", "Onde você estudou?"},
		"marriage":  {"Como você conheceu seu esposo/esposa?", "Como foi o casamento?"},
	}
	if q, ok := questions[theme]; ok {
		return q
	}
	return []string{"Me conte sobre essa época da sua vida..."}
}

func (h *ToolsHandler) getSongForReminiscence(idosoID int64, theme string) *Song {
	return &Song{Title: "Aquarela do Brasil", Artist: "Ary Barroso"}
}

func (h *ToolsHandler) fetchNews(category, source, detailLevel string) []*NewsArticle {
	return []*NewsArticle{
		{Title: "Boa notícia do dia", Summary: "Algo positivo aconteceu", Source: "G1"},
	}
}

func formatNewsForReading(news []*NewsArticle, level string) string {
	if len(news) == 0 {
		return "Não encontrei notícias no momento."
	}
	return fmt.Sprintf("Principal notícia: %s", news[0].Title)
}

func (h *ToolsHandler) getPatientSign(idosoID int64) string {
	return "leo"
}

func getHoroscope(sign string) string {
	horoscopes := map[string]string{
		"leo": "Hoje é um dia de luz e energia positiva. Aproveite para se conectar com pessoas queridas.",
	}
	if h, ok := horoscopes[sign]; ok {
		return h
	}
	return "Hoje é um dia especial. Aproveite cada momento com gratidão."
}

func getSignName(sign string) string {
	names := map[string]string{
		"aries": "Áries", "taurus": "Touro", "gemini": "Gêmeos",
		"cancer": "Câncer", "leo": "Leão", "virgo": "Virgem",
		"libra": "Libra", "scorpio": "Escorpião", "sagittarius": "Sagitário",
		"capricorn": "Capricórnio", "aquarius": "Aquário", "pisces": "Peixes",
	}
	return names[sign]
}

func (h *ToolsHandler) getPatientLocation(idosoID int64) string {
	return "São Paulo"
}

func (h *ToolsHandler) getWeatherForecast(location, forecastType string) *Weather {
	return &Weather{Temperature: 25, Condition: "Ensolarado", Humidity: 60}
}

func generateWeatherTips(weather *Weather) []string {
	if weather.Temperature > 30 {
		return []string{"Beba bastante água", "Evite sair no sol forte"}
	}
	if weather.Temperature < 15 {
		return []string{"Vista um casaco", "Tome um chá quente"}
	}
	return []string{"Dia agradável para um passeio"}
}

func formatWeatherMessage(weather *Weather, tips []string) string {
	return fmt.Sprintf("Está %d graus, %s. %s", weather.Temperature, weather.Condition, tips[0])
}

func (h *ToolsHandler) searchRecipes(dishType, difficulty string) []*Recipe {
	return []*Recipe{{Name: "Bolo de Cenoura", Ingredients: []string{"cenoura", "ovos", "açúcar"}}}
}

func (h *ToolsHandler) getRecipe(name string) *Recipe {
	return &Recipe{
		Name:        "Bolo de Cenoura",
		Ingredients: []string{"3 cenouras", "3 ovos", "1 xícara de açúcar", "1/2 xícara de óleo", "2 xícaras de farinha"},
		Steps:       []string{"Bata as cenouras com os ovos no liquidificador", "Adicione o açúcar e o óleo", "Misture a farinha", "Asse por 40 minutos"},
	}
}

func (h *ToolsHandler) saveRecipeState(idosoID int64, recipe *Recipe, step int) {}

func (h *ToolsHandler) getNextRecipeStep(idosoID int64) (string, bool) {
	return "Adicione o açúcar e o óleo", false
}

func (h *ToolsHandler) getCurrentRecipeStep(idosoID int64) string {
	return "Bata as cenouras com os ovos no liquidificador"
}

func (h *ToolsHandler) getRecipeIngredients(idosoID int64) []string {
	return []string{"3 cenouras", "3 ovos", "1 xícara de açúcar"}
}

func (h *ToolsHandler) getDiaryEntries(idosoID int64, period string) []*DiaryEntry {
	return []*DiaryEntry{}
}

func (h *ToolsHandler) getDiaryEntriesByDate(idosoID int64, date string) []*DiaryEntry {
	return []*DiaryEntry{}
}

func (h *ToolsHandler) getRandomDiaryEntry(idosoID int64) *DiaryEntry {
	return &DiaryEntry{Date: "15/01/2026", Preview: "Hoje foi um bom dia..."}
}

func (h *ToolsHandler) getRecentDiaryEntries(idosoID int64, limit int) []*DiaryEntry {
	return []*DiaryEntry{}
}

// Funções utilitárias
func joinSequence(seq []string) string {
	result := ""
	for i, s := range seq {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

func joinEntries(entries []string) string {
	return joinSequence(entries)
}

func joinNames(birthdays []*Birthday) string {
	names := make([]string, len(birthdays))
	for i, b := range birthdays {
		names[i] = b.Name
	}
	return joinSequence(names)
}

func joinIngredients(ingredients []string) string {
	return joinSequence(ingredients)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// =============================================================================
// FUNÇÕES AUXILIARES - NOVAS TOOLS
// =============================================================================

// Audiobook helpers
func (h *ToolsHandler) getAudiobook(title string) *Audiobook {
	return &Audiobook{
		ID:        "book-001",
		Title:     "Dom Casmurro",
		Author:    "Machado de Assis",
		StreamURL: "https://audio.example.com/dom-casmurro.mp3",
	}
}

func (h *ToolsHandler) listAvailableAudiobooks() []*Audiobook {
	return []*Audiobook{
		{ID: "book-001", Title: "Dom Casmurro", Author: "Machado de Assis"},
		{ID: "book-002", Title: "O Cortiço", Author: "Aluísio Azevedo"},
	}
}

func (h *ToolsHandler) searchAudiobooks(query string) []*Audiobook {
	return h.listAvailableAudiobooks()
}

// Podcast helpers
func (h *ToolsHandler) getPodcastByCategory(category string) *Podcast {
	return &Podcast{
		ID:        "pod-001",
		Title:     "Histórias do Brasil",
		Episode:   "Episódio 42: A Era de Ouro do Rádio",
		StreamURL: "https://podcast.example.com/ep42.mp3",
	}
}

func (h *ToolsHandler) listPodcastsByCategory(category string) []*Podcast {
	return []*Podcast{
		{ID: "pod-001", Title: "Histórias do Brasil", Episode: "Episódio 42"},
	}
}

func (h *ToolsHandler) searchPodcasts(query string) []*Podcast {
	return h.listPodcastsByCategory("")
}

// Word association helpers
func (h *ToolsHandler) getWordForAssociation(category string) string {
	words := map[string][]string{
		"general": {"casa", "amor", "família", "sol", "música"},
		"food":    {"café", "pão", "arroz", "feijão", "bolo"},
		"places":  {"praia", "montanha", "cidade", "campo", "rio"},
	}
	if w, ok := words[category]; ok {
		return w[rand.Intn(len(w))]
	}
	return "vida"
}

func (h *ToolsHandler) saveWordGameState(idosoID int64, word string) {}

func (h *ToolsHandler) processWordResponse(idosoID int64, response string) (bool, string, string) {
	nextWord := h.getWordForAssociation("general")
	return true, "Boa associação!", nextWord
}

func (h *ToolsHandler) getWordAssociationScore(idosoID int64) int {
	return 10
}

// Lyric challenge helpers
func (h *ToolsHandler) getLyricChallenge(decade string) *LyricChallenge {
	return &LyricChallenge{
		SongTitle:    "Aquarela do Brasil",
		Artist:       "Ary Barroso",
		PartialLyric: "Brasil, meu Brasil brasileiro, meu mulato...",
		FullLyric:    "Brasil, meu Brasil brasileiro, meu mulato inzoneiro",
	}
}

func (h *ToolsHandler) saveLyricGameState(idosoID int64, lyric *LyricChallenge) {}

func (h *ToolsHandler) checkLyricAnswer(idosoID int64, answer string) (bool, string) {
	return true, "Isso mesmo! 'meu mulato inzoneiro'"
}

func (h *ToolsHandler) skipLyricChallenge(idosoID int64) (string, *LyricChallenge) {
	return "meu mulato inzoneiro", h.getLyricChallenge("1950s")
}

func (h *ToolsHandler) getLyricHint(idosoID int64) string {
	return "Começa com 'meu mulato...'"
}

func (h *ToolsHandler) getLyricScore(idosoID int64) *TriviaScore {
	return &TriviaScore{Correct: 7, Total: 10}
}

// Sleep story helpers
func (h *ToolsHandler) getSleepStory(theme string, includeBreathing bool) *SleepStory {
	return &SleepStory{
		ID:              "sleep-001",
		Title:           "A Viagem pelas Nuvens",
		AudioURL:        "https://audio.example.com/sleep-clouds.mp3",
		DurationMinutes: 20,
	}
}

// Motivational quote helpers
func (h *ToolsHandler) getMotivationalQuote(theme, authorType string) *MotivationalQuote {
	quotes := []MotivationalQuote{
		{Text: "A felicidade não está em viver, mas em saber viver.", Author: "Cora Coralina"},
		{Text: "Não há nada de errado em ser velho, desde que você seja jovem.", Author: "Santa Teresa de Calcutá"},
		{Text: "Cada dia é uma nova chance de ser feliz.", Author: "Autor Desconhecido"},
	}
	return &quotes[rand.Intn(len(quotes))]
}

// Family tree helpers
func (h *ToolsHandler) getFamilyTree(idosoID int64) *FamilyTree {
	return &FamilyTree{
		Members: []FamilyMember{
			{Name: "Maria", Relation: "filha"},
			{Name: "João", Relation: "filho"},
			{Name: "Ana", Relation: "neta"},
		},
	}
}

func (h *ToolsHandler) addFamilyMember(idosoID int64, name, relation string) {}

func (h *ToolsHandler) addFamilyStory(idosoID int64, personName, story string) {}

func (h *ToolsHandler) findFamilyRelation(idosoID int64, personName string) string {
	return "filho"
}

// Photo slideshow helpers
func (h *ToolsHandler) getPhotoAlbum(idosoID int64, album string) []*Photo {
	return []*Photo{
		{URL: "https://photos.example.com/1.jpg", Description: "Casamento 1975", Date: "15/06/1975"},
		{URL: "https://photos.example.com/2.jpg", Description: "Nascimento do filho", Date: "20/03/1978"},
	}
}

func (h *ToolsHandler) getNextPhoto(idosoID int64) *Photo {
	return &Photo{URL: "https://photos.example.com/2.jpg", Description: "Nascimento do filho", Date: "20/03/1978"}
}

func (h *ToolsHandler) getPreviousPhoto(idosoID int64) *Photo {
	return &Photo{URL: "https://photos.example.com/1.jpg", Description: "Casamento 1975", Date: "15/06/1975"}
}

func (h *ToolsHandler) savePhotoComment(idosoID int64, comment string) {}

// Biography writer helpers
func (h *ToolsHandler) getBiographyQuestions(lifeChapter string) []string {
	questions := map[string][]string{
		"birth_childhood": {"Onde você nasceu?", "Como era sua infância?", "Quais são suas primeiras lembranças?"},
		"youth":           {"O que você gostava de fazer quando jovem?", "Quais eram seus sonhos?"},
		"love_marriage":   {"Como você conheceu o amor da sua vida?", "Como foi o namoro?", "E o casamento?"},
		"career":          {"Qual foi sua profissão?", "O que mais te orgulha na sua carreira?"},
		"parenthood":      {"Como foi se tornar pai/mãe?", "Quais valores você quis passar?"},
		"wisdom":          {"O que a vida te ensinou?", "Que conselho você daria aos jovens?"},
		"legacy":          {"Como você gostaria de ser lembrado?", "Qual mensagem deixaria para sua família?"},
	}
	if q, ok := questions[lifeChapter]; ok {
		return q
	}
	return []string{"Me conte sobre essa fase da sua vida..."}
}

func (h *ToolsHandler) saveBiographySession(idosoID int64, chapter string) {}

func (h *ToolsHandler) getNextBiographyQuestion(idosoID int64) string {
	return "E o que aconteceu depois?"
}

func (h *ToolsHandler) getCurrentBiography(idosoID int64) string {
	return "Nascido em uma pequena cidade do interior..."
}

func (h *ToolsHandler) exportBiography(idosoID int64) string {
	return fmt.Sprintf("https://eva.example.com/biography/%d/export", idosoID)
}
