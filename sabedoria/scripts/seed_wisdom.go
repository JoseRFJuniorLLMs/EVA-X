package main

import (
	"context"
	"eva/internal/brainstem/config"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/hippocampus/memory"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// WisdomSource define uma fonte de sabedoria
type WisdomSource struct {
	Name       string // Nome para CLI
	File       string // Caminho do arquivo
	Collection string // Nome da coleção no Qdrant
	Tradition  string // Tradição (sufi, quarto_caminho, zen, etc)
	Type       string // Tipo (teaching, story, exercise, poem, koan)
	Tags       []string
}

// Todas as fontes de sabedoria disponíveis
var WisdomSources = []WisdomSource{
	// Mestres do Criador
	{
		Name:       "gurdjieff",
		File:       "docs/conhecimento/GURDJIEFF_TEACHINGS.txt",
		Collection: "gurdjieff_teachings",
		Tradition:  "quarto_caminho",
		Type:       "teaching",
		Tags:       []string{"auto-observação", "despertar", "trabalho", "consciência"},
	},
	{
		Name:       "osho",
		File:       "docs/conhecimento/OSHO_INSIGHTS.txt",
		Collection: "osho_insights",
		Tradition:  "osho",
		Type:       "insight",
		Tags:       []string{"testemunho", "meditação", "celebração", "provocação"},
	},
	{
		Name:       "ouspensky",
		File:       "docs/conhecimento/OUSPENSKY_FRAGMENTS.txt",
		Collection: "ouspensky_fragments",
		Tradition:  "quarto_caminho",
		Type:       "teaching",
		Tags:       []string{"máquina", "centros", "tipos", "desenvolvimento"},
	},
	{
		Name:       "nietzsche",
		File:       "docs/conhecimento/NIETZSCHE_ZARATUSTRA.txt",
		Collection: "nietzsche_aphorisms",
		Tradition:  "filosofia",
		Type:       "aphorism",
		Tags:       []string{"super-homem", "vontade", "força", "transvaloração"},
	},
	// Tradições
	{
		Name:       "zen",
		File:       "docs/conhecimento/ZEN_KOANS.txt",
		Collection: "zen_koans",
		Tradition:  "zen",
		Type:       "koan",
		Tags:       []string{"paradoxo", "não-mente", "iluminação", "presente"},
	},
	{
		Name:       "rumi",
		File:       "docs/conhecimento/RUMI_POEMS.txt",
		Collection: "rumi_poems",
		Tradition:  "sufi",
		Type:       "poem",
		Tags:       []string{"amor", "união", "divino", "coração"},
	},
	{
		Name:       "estoicos",
		File:       "docs/conhecimento/STOIC_MEDITATIONS.txt",
		Collection: "stoic_meditations",
		Tradition:  "estoica",
		Type:       "meditation",
		Tags:       []string{"aceitação", "controle", "virtude", "resiliência"},
	},
	// Técnicas
	{
		Name:       "osho_med",
		File:       "docs/conhecimento/OSHO_MEDITATIONS.txt",
		Collection: "osho_meditations",
		Tradition:  "osho",
		Type:       "meditation",
		Tags:       []string{"ativa", "catarse", "energia", "silêncio"},
	},
	{
		Name:       "respiracao",
		File:       "docs/conhecimento/BREATHING_SCRIPTS.txt",
		Collection: "breathing_scripts",
		Tradition:  "integrativa",
		Type:       "exercise",
		Tags:       []string{"respiração", "regulação", "calma", "energia"},
	},
	{
		Name:       "hipnose",
		File:       "docs/conhecimento/SELF_HYPNOSIS_SCRIPTS.txt",
		Collection: "hypnosis_scripts",
		Tradition:  "hipnoterapia",
		Type:       "script",
		Tags:       []string{"autoindução", "relaxamento", "reprogramação", "transe"},
	},
	{
		Name:       "somatico",
		File:       "docs/conhecimento/SOMATIC_EXERCISES.txt",
		Collection: "somatic_exercises",
		Tradition:  "somática",
		Type:       "exercise",
		Tags:       []string{"corpo", "grounding", "regulação", "trauma"},
	},
	{
		Name:       "gestalt",
		File:       "docs/conhecimento/GESTALT_EXERCISES.txt",
		Collection: "gestalt_exercises",
		Tradition:  "gestalt",
		Type:       "exercise",
		Tags:       []string{"awareness", "aqui-agora", "contato", "polaridades"},
	},
	{
		Name:       "wimhof",
		File:       "docs/conhecimento/WIM_HOF_PROTOCOLS.txt",
		Collection: "wim_hof_protocols",
		Tradition:  "wim_hof",
		Type:       "protocol",
		Tags:       []string{"respiração", "frio", "foco", "energia"},
	},
	// Psicologia
	{
		Name:       "jung",
		File:       "docs/conhecimento/JUNG_ARCHETYPES.txt",
		Collection: "jung_archetypes",
		Tradition:  "junguiana",
		Type:       "concept",
		Tags:       []string{"arquétipo", "sombra", "individuação", "inconsciente"},
	},
	// Já existentes em docs/
	{
		Name:       "nasrudin",
		File:       "docs/NASRUDIN_STORIES.txt",
		Collection: "nasrudin_stories",
		Tradition:  "sufi",
		Type:       "story",
		Tags:       []string{"humor", "paradoxo", "sabedoria", "sufi"},
	},
	{
		Name:       "esopo",
		File:       "docs/FABULAS_ESOPO.txt",
		Collection: "aesop_fables",
		Tradition:  "grega",
		Type:       "fable",
		Tags:       []string{"moral", "animais", "lição", "fábula"},
	},
}

func main() {
	log.Println("🌱 EVA Wisdom Seeder - Base de Conhecimento (3072 dims)")
	log.Println("═══════════════════════════════════════════════════════════")

	// Carregar .env explicitamente (pode estar em diretório diferente com go run)
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️ Não encontrou .env no diretório atual, tentando caminhos alternativos...")
		// Tenta caminhos comuns
		paths := []string{".env", "../.env", "../../.env"}
		loaded := false
		for _, p := range paths {
			if err := godotenv.Load(p); err == nil {
				log.Printf("✅ Carregado .env de: %s", p)
				loaded = true
				break
			}
		}
		if !loaded {
			log.Println("⚠️ .env não encontrado, usando variáveis de ambiente do sistema")
		}
	}

	// Debug: mostrar diretório atual
	cwd, _ := os.Getwd()
	log.Printf("📂 Diretório atual: %s", cwd)

	// Carregar Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Config error: %v", err)
	}

	// Verificar credenciais (Vertex AI token tem prioridade)
	vertexToken := os.Getenv("VERTEX_ACCESS_TOKEN")
	if vertexToken != "" {
		log.Printf("🔐 Vertex AI Token: %s...%s (len=%d)", vertexToken[:8], vertexToken[len(vertexToken)-4:], len(vertexToken))
	} else if cfg.GoogleAPIKey != "" {
		log.Printf("🔑 API Key: %s...%s (len=%d)", cfg.GoogleAPIKey[:8], cfg.GoogleAPIKey[len(cfg.GoogleAPIKey)-4:], len(cfg.GoogleAPIKey))
	} else {
		log.Fatal("❌ Nenhuma credencial encontrada (VERTEX_ACCESS_TOKEN ou GOOGLE_API_KEY)")
	}

	// Conectar NietzscheDB
	nietzscheAddr := cfg.NietzscheGRPCAddr
	if nietzscheAddr == "" {
		nietzscheAddr = "localhost:50051"
	}
	log.Printf("🔌 Conectando ao NietzscheDB: %s", nietzscheAddr)

	nietzscheClient, err := nietzscheInfra.NewClient(nietzscheAddr)
	if err != nil {
		log.Fatalf("❌ Erro ao conectar NietzscheDB: %v", err)
	}
	defer nietzscheClient.Close()
	vAdapter := nietzscheInfra.NewVectorAdapter(nietzscheClient)

	// Criar embedder (detecta automaticamente Vertex AI ou API Key)
	embedder := memory.NewEmbeddingServiceFromEnv()
	ctx := context.Background()

	// Processar argumentos
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	source := strings.ToLower(os.Args[1])

	switch source {
	case "all":
		seedAll(ctx, vAdapter, embedder)
	case "list":
		listSources()
	case "status":
		checkStatus(ctx, vAdapter)
	default:
		// Buscar fonte específica
		found := false
		for _, ws := range WisdomSources {
			if ws.Name == source {
				seedSource(ctx, vAdapter, embedder, ws)
				found = true
				break
			}
		}
		if !found {
			log.Printf("❌ Fonte desconhecida: %s", source)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println("\n📚 EVA Wisdom Seeder")
	fmt.Println("════════════════════")
	fmt.Println("\nUso: seed_wisdom <comando>")
	fmt.Println("\nComandos:")
	fmt.Println("  all      - Seed de todas as fontes")
	fmt.Println("  list     - Lista todas as fontes disponíveis")
	fmt.Println("  status   - Verifica status das coleções no NietzscheDB")
	fmt.Println("  <fonte>  - Seed de uma fonte específica")
	fmt.Println("\nFontes disponíveis:")
	for _, ws := range WisdomSources {
		fmt.Printf("  %-12s → %s\n", ws.Name, ws.Collection)
	}
}

func listSources() {
	fmt.Println("\n📚 Fontes de Sabedoria Disponíveis")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("%-12s │ %-25s │ %-15s │ %s\n", "NOME", "COLEÇÃO", "TRADIÇÃO", "ARQUIVO")
	fmt.Println("─────────────┼───────────────────────────┼─────────────────┼────────────────────────────────")
	for _, ws := range WisdomSources {
		fmt.Printf("%-12s │ %-25s │ %-15s │ %s\n", ws.Name, ws.Collection, ws.Tradition, ws.File)
	}
}

func checkStatus(_ context.Context, _ *nietzscheInfra.VectorAdapter) {
	fmt.Println("\n📊 Status das Coleções no NietzscheDB")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("%-25s │ %s\n", "COLEÇÃO", "STATUS")
	fmt.Println("──────────────────────────┼────────────────────────────────")

	for _, ws := range WisdomSources {
		fmt.Printf("%-25s │ ℹ️  (use NietzscheDB dashboard)\n", ws.Collection)
	}
}

func seedAll(ctx context.Context, vAdapter *nietzscheInfra.VectorAdapter, embedder *memory.EmbeddingService) {
	log.Println("🚀 Iniciando seed de TODAS as fontes...")
	log.Println("")

	for _, ws := range WisdomSources {
		seedSource(ctx, vAdapter, embedder, ws)
		log.Println("")
	}

	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("🎉 Seed completo de todas as fontes!")
}

func seedSource(ctx context.Context, vAdapter *nietzscheInfra.VectorAdapter, embedder *memory.EmbeddingService, ws WisdomSource) {
	log.Printf("📖 [%s] Processando %s...", ws.Name, ws.File)

	content, err := os.ReadFile(ws.File)
	if err != nil {
		log.Printf("❌ [%s] Erro ao ler arquivo: %v", ws.Name, err)
		return
	}

	entries := parseEntries(string(content))
	log.Printf("📚 [%s] Encontradas %d entradas", ws.Name, len(entries))

	if len(entries) == 0 {
		log.Printf("⚠️ [%s] Nenhuma entrada encontrada!", ws.Name)
		return
	}

	totalProcessed := 0
	for i, entry := range entries {
		if len(entry) < 10 {
			continue
		}

		textToEmbed := fmt.Sprintf("%s (%s): %s", ws.Tradition, ws.Type, entry)
		vec, err := embedder.GenerateEmbedding(ctx, textToEmbed)
		if err != nil {
			log.Printf("⚠️ [%s] Erro embedding entrada %d: %v", ws.Name, i+1, err)
			continue
		}

		pointID := fmt.Sprintf("%s_%d", ws.Name, i+1)
		payload := map[string]interface{}{
			"id":        pointID,
			"content":   entry,
			"source":    ws.Name,
			"tradition": ws.Tradition,
			"type":      ws.Type,
			"tags":      strings.Join(ws.Tags, ","),
		}

		if err := vAdapter.Upsert(ctx, ws.Collection, pointID, vec, payload); err != nil {
			log.Printf("❌ [%s] Erro no upsert entrada %d: %v", ws.Name, i+1, err)
		} else {
			totalProcessed++
			if totalProcessed%10 == 0 {
				log.Printf("✅ [%s] %d entradas processadas...", ws.Name, totalProcessed)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	log.Printf("🎉 [%s] Completo! %d entradas em '%s'", ws.Name, totalProcessed, ws.Collection)
}

// parseEntries extrai entradas numeradas de um arquivo
func parseEntries(content string) []string {
	var entries []string

	// Remove linhas de placeholder como "[... arquivo completo contém X entradas]"
	placeholderRe := regexp.MustCompile(`\[\.\.\..*?\]`)
	content = placeholderRe.ReplaceAllString(content, "")

	// Remove linhas decorativas
	decorRe := regexp.MustCompile(`[━═─]+`)
	content = decorRe.ReplaceAllString(content, "\n")

	// Dividir por linhas
	lines := strings.Split(content, "\n")

	// Regex para detectar início de entrada numerada
	numRe := regexp.MustCompile(`^\s*(\d+)\.\s+(.+)`)

	var currentEntry strings.Builder
	inEntry := false

	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")

		// Verifica se é nova entrada numerada
		if match := numRe.FindStringSubmatch(line); match != nil {
			// Salvar entrada anterior se existir
			if inEntry && currentEntry.Len() > 0 {
				text := strings.TrimSpace(currentEntry.String())
				text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
				if len(text) >= 10 {
					entries = append(entries, text)
				}
			}

			// Iniciar nova entrada
			currentEntry.Reset()
			currentEntry.WriteString(match[2])
			inEntry = true
		} else if inEntry && len(line) > 0 {
			// Continuar entrada atual (linha não vazia)
			currentEntry.WriteString(" ")
			currentEntry.WriteString(line)
		}
	}

	// Salvar última entrada
	if inEntry && currentEntry.Len() > 0 {
		text := strings.TrimSpace(currentEntry.String())
		text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
		if len(text) >= 10 {
			entries = append(entries, text)
		}
	}

	return entries
}

