package entertainment

import (
	"context"
	"fmt"
	"log"

	"eva-mind/internal/swarm"
)

// Agent implementa o EntertainmentSwarm - música, jogos, mídia, espiritual
type Agent struct {
	*swarm.BaseAgent
}

// New cria o EntertainmentSwarm agent
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"entertainment",
			"Música, jogos, filmes, notícias, espiritual, humor",
			swarm.PriorityLow,
		),
	}
	a.registerTools()
	return a
}

func (a *Agent) registerTools() {
	// --- MÚSICA ---
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "play_nostalgic_music",
		Description: "Toca músicas da época de ouro (juventude) do paciente",
		Parameters: map[string]interface{}{
			"decade": map[string]interface{}{
				"type": "string", "description": "Década preferida (ex: 'anos 60')",
			},
		},
	}, a.genericHandler("music", "Tocando música nostálgica"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "radio_station_tuner",
		Description: "Sintoniza estações de rádio AM/FM via streaming",
		Parameters: map[string]interface{}{
			"station_name": map[string]interface{}{
				"type": "string", "description": "Nome da rádio",
			},
		},
		Required: []string{"station_name"},
	}, a.genericHandler("radio", "Sintonizando rádio"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "play_relaxation_sounds",
		Description: "Toca sons de natureza ou white noise para relaxamento",
		Parameters: map[string]interface{}{
			"sound_type": map[string]interface{}{
				"type": "string", "description": "Tipo de som",
				"enum": []string{"chuva", "mar", "floresta", "lareira", "sino_tibetano"},
			},
		},
		Required: []string{"sound_type"},
	}, a.genericHandler("relaxation", "Tocando sons relaxantes"))

	// --- ESPIRITUAL ---
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "hymn_and_prayer_player",
		Description: "Reproduz hinos religiosos ou guia orações",
		Parameters: map[string]interface{}{
			"type":         map[string]interface{}{"type": "string", "description": "Tipo: hino, oracao, terço, salmo"},
			"content_name": map[string]interface{}{"type": "string", "description": "Nome específico"},
		},
		Required: []string{"type"},
	}, a.genericHandler("spiritual", "Reproduzindo conteúdo espiritual"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "daily_mass_stream",
		Description: "Inicia transmissão de missa ao vivo ou gravada",
		Parameters:  map[string]interface{}{},
	}, a.genericHandler("spiritual", "Iniciando missa"))

	// --- MÍDIA ---
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "watch_classic_movies",
		Description: "Busca e reproduz filmes clássicos",
		Parameters: map[string]interface{}{
			"movie_name": map[string]interface{}{"type": "string", "description": "Nome do filme ou ator"},
		},
		Required: []string{"movie_name"},
	}, a.genericHandler("media", "Buscando filme"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "watch_news_briefing",
		Description: "Apresenta resumo das notícias do dia",
		Parameters: map[string]interface{}{
			"topic": map[string]interface{}{"type": "string", "description": "Tópico (geral, política, esportes)"},
		},
	}, a.genericHandler("media", "Preparando resumo de notícias"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "read_newspaper_aloud",
		Description: "Lê manchetes dos principais jornais",
		Parameters: map[string]interface{}{
			"newspaper": map[string]interface{}{"type": "string", "description": "Nome do jornal"},
		},
	}, a.genericHandler("media", "Lendo manchetes"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "horoscope_daily",
		Description: "Busca e lê o horóscopo do dia",
		Parameters: map[string]interface{}{
			"sign": map[string]interface{}{"type": "string", "description": "Signo"},
		},
	}, a.genericHandler("media", "Buscando horóscopo"))

	// --- JOGOS ---
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "play_trivia_game",
		Description: "Inicia jogo de perguntas e respostas",
		Parameters: map[string]interface{}{
			"theme": map[string]interface{}{"type": "string", "description": "Tema do quiz"},
		},
	}, a.genericHandler("games", "Iniciando trivia"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "memory_game",
		Description: "Jogo de memória",
		Parameters: map[string]interface{}{
			"difficulty": map[string]interface{}{"type": "string", "description": "Dificuldade"},
		},
	}, a.genericHandler("games", "Iniciando jogo de memória"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "word_association",
		Description: "Jogo de associação de palavras",
		Parameters:  map[string]interface{}{},
	}, a.genericHandler("games", "Iniciando associação de palavras"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "brain_training",
		Description: "Exercícios cognitivos",
		Parameters: map[string]interface{}{
			"type": map[string]interface{}{"type": "string", "description": "Tipo de treino"},
		},
	}, a.genericHandler("games", "Iniciando treino cognitivo"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "riddle_and_joke_teller",
		Description: "Conta piadas ou propõe charadas",
		Parameters: map[string]interface{}{
			"type": map[string]interface{}{
				"type": "string", "description": "piada ou adivinha",
				"enum": []string{"piada", "adivinha"},
			},
		},
	}, a.genericHandler("humor", "Preparando humor"))

	// --- CRIATIVO ---
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "poetry_generator",
		Description: "Cria um poema personalizado",
		Parameters: map[string]interface{}{
			"theme": map[string]interface{}{"type": "string", "description": "Tema do poema"},
		},
	}, a.genericHandler("creative", "Criando poema"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "learn_new_language",
		Description: "Inicia lição de idioma",
		Parameters: map[string]interface{}{
			"language": map[string]interface{}{
				"type": "string", "description": "Idioma",
				"enum": []string{"ingles", "espanhol", "frances"},
			},
		},
		Required: []string{"language"},
	}, a.genericHandler("education", "Iniciando lição"))

	// --- DIARY & STORIES ---
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "voice_diary",
		Description: "Inicia sessão de diário por voz",
		Parameters:  map[string]interface{}{},
	}, a.genericHandler("diary", "Iniciando diário por voz"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "story_generator",
		Description: "Gerar história personalizada",
		Parameters: map[string]interface{}{
			"theme":      map[string]interface{}{"type": "string", "description": "Tema"},
			"characters": map[string]interface{}{"type": "string", "description": "Personagens"},
		},
	}, a.genericHandler("creative", "Criando história"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "reminiscence_therapy",
		Description: "Terapia de reminiscência",
		Parameters: map[string]interface{}{
			"era":   map[string]interface{}{"type": "string", "description": "Era/década"},
			"topic": map[string]interface{}{"type": "string", "description": "Tema"},
		},
	}, a.genericHandler("therapy", "Iniciando reminiscência"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "biography_writer",
		Description: "Escrever biografia do idoso",
		Parameters: map[string]interface{}{
			"life_period": map[string]interface{}{"type": "string", "description": "Período da vida"},
		},
	}, a.genericHandler("creative", "Escrevendo biografia"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "voice_capsule",
		Description: "Gravar cápsula do tempo por voz",
		Parameters: map[string]interface{}{
			"recipient": map[string]interface{}{"type": "string", "description": "Destinatário"},
		},
	}, a.genericHandler("memory", "Gravando cápsula do tempo"))

	// --- FAMÍLIA ---
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "birthday_reminder",
		Description: "Lembrar aniversários da família",
		Parameters:  map[string]interface{}{},
	}, a.genericHandler("family", "Verificando aniversários"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "family_tree_explorer",
		Description: "Explorar árvore genealógica",
		Parameters:  map[string]interface{}{},
	}, a.genericHandler("family", "Explorando árvore genealógica"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "photo_slideshow",
		Description: "Mostrar fotos da família",
		Parameters:  map[string]interface{}{},
	}, a.genericHandler("family", "Preparando slideshow"))

	// --- UTILIDADES ---
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "weather_chat",
		Description: "Conversar sobre o tempo",
		Parameters: map[string]interface{}{
			"location": map[string]interface{}{"type": "string", "description": "Localização"},
		},
	}, a.genericHandler("utility", "Verificando previsão do tempo"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "cooking_recipes",
		Description: "Receitas de culinária",
		Parameters: map[string]interface{}{
			"dish_type": map[string]interface{}{"type": "string", "description": "Tipo de prato"},
		},
	}, a.genericHandler("utility", "Buscando receitas"))

	// --- BEM-ESTAR EXTRAS ---
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "sleep_stories",
		Description: "Histórias para dormir",
		Parameters: map[string]interface{}{
			"theme": map[string]interface{}{"type": "string", "description": "Tema"},
		},
	}, a.genericHandler("wellness", "Preparando história para dormir"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "gratitude_journal",
		Description: "Diário de gratidão",
		Parameters:  map[string]interface{}{},
	}, a.genericHandler("wellness", "Iniciando diário de gratidão"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "motivational_quotes",
		Description: "Frases motivacionais",
		Parameters: map[string]interface{}{
			"theme": map[string]interface{}{"type": "string", "description": "Tema"},
		},
	}, a.genericHandler("wellness", "Buscando frase motivacional"))
}

func (a *Agent) genericHandler(category, message string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("🎭 [ENTERTAINMENT:%s] %s → %s userID=%d", category, call.Name, message, call.UserID)

		return &swarm.ToolResult{
			Success:     true,
			Message:     message,
			SuggestTone: "alegre_acolhedor",
			Data: map[string]interface{}{
				"action":   call.Name,
				"category": category,
				"args":     call.Args,
				"user_id":  call.UserID,
			},
		}, nil
	}
}

// ToolCount retorna o número de tools (helper para log)
func (a *Agent) ToolCount() int {
	return len(a.Tools())
}
