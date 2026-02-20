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
	Collection string // Nome da colecao no NietzscheDB
	Tradition  string // Tradicao (sufi, quarto_caminho, zen, etc)
	Type       string // Tipo (teaching, story, exercise, poem, koan)
	Tags       []string
}

// Todas as fontes de sabedoria disponiveis
var WisdomSources = []WisdomSource{
	// Mestres do Criador
	{
		Name:       "gurdjieff",
		File:       "sabedoria/conhecimento/GURDJIEFF_TEACHINGS.txt",
		Collection: "gurdjieff_teachings",
		Tradition:  "quarto_caminho",
		Type:       "teaching",
		Tags:       []string{"auto-observacao", "despertar", "trabalho", "consciencia"},
	},
	{
		Name:       "osho",
		File:       "sabedoria/conhecimento/OSHO_INSIGHTS.txt",
		Collection: "osho_insights",
		Tradition:  "osho",
		Type:       "insight",
		Tags:       []string{"testemunho", "meditacao", "celebracao", "provocacao"},
	},
	{
		Name:       "ouspensky",
		File:       "sabedoria/conhecimento/OUSPENSKY_FRAGMENTS.txt",
		Collection: "ouspensky_fragments",
		Tradition:  "quarto_caminho",
		Type:       "teaching",
		Tags:       []string{"maquina", "centros", "tipos", "desenvolvimento"},
	},
	{
		Name:       "nietzsche",
		File:       "sabedoria/conhecimento/NIETZSCHE_ZARATUSTRA.txt",
		Collection: "nietzsche_aphorisms",
		Tradition:  "filosofia",
		Type:       "aphorism",
		Tags:       []string{"super-homem", "vontade", "forca", "transvaloracao"},
	},
	// Tradicoes
	{
		Name:       "zen",
		File:       "sabedoria/conhecimento/ZEN_KOANS.txt",
		Collection: "zen_koans",
		Tradition:  "zen",
		Type:       "koan",
		Tags:       []string{"paradoxo", "nao-mente", "iluminacao", "presente"},
	},
	{
		Name:       "rumi",
		File:       "sabedoria/conhecimento/RUMI_POEMS.txt",
		Collection: "rumi_poems",
		Tradition:  "sufi",
		Type:       "poem",
		Tags:       []string{"amor", "uniao", "divino", "coracao"},
	},
	{
		Name:       "estoicos",
		File:       "sabedoria/conhecimento/STOIC_MEDITATIONS.txt",
		Collection: "stoic_meditations",
		Tradition:  "estoica",
		Type:       "meditation",
		Tags:       []string{"aceitacao", "controle", "virtude", "resiliencia"},
	},
	// Tecnicas
	{
		Name:       "osho_med",
		File:       "sabedoria/conhecimento/OSHO_MEDITATIONS.txt",
		Collection: "osho_meditations",
		Tradition:  "osho",
		Type:       "meditation",
		Tags:       []string{"ativa", "catarse", "energia", "silencio"},
	},
	{
		Name:       "respiracao",
		File:       "sabedoria/conhecimento/BREATHING_SCRIPTS.txt",
		Collection: "breathing_scripts",
		Tradition:  "integrativa",
		Type:       "exercise",
		Tags:       []string{"respiracao", "regulacao", "calma", "energia"},
	},
	{
		Name:       "hipnose",
		File:       "sabedoria/conhecimento/SELF_HYPNOSIS_SCRIPTS.txt",
		Collection: "hypnosis_scripts",
		Tradition:  "hipnoterapia",
		Type:       "script",
		Tags:       []string{"autoinducao", "relaxamento", "reprogramacao", "transe"},
	},
	{
		Name:       "somatico",
		File:       "sabedoria/conhecimento/SOMATIC_EXERCISES.txt",
		Collection: "somatic_exercises",
		Tradition:  "somatica",
		Type:       "exercise",
		Tags:       []string{"corpo", "grounding", "regulacao", "trauma"},
	},
	{
		Name:       "gestalt",
		File:       "sabedoria/conhecimento/GESTALT_EXERCISES.txt",
		Collection: "gestalt_exercises",
		Tradition:  "gestalt",
		Type:       "exercise",
		Tags:       []string{"awareness", "aqui-agora", "contato", "polaridades"},
	},
	{
		Name:       "wimhof",
		File:       "sabedoria/conhecimento/WIM_HOF_PROTOCOLS.txt",
		Collection: "wim_hof_protocols",
		Tradition:  "wim_hof",
		Type:       "protocol",
		Tags:       []string{"respiracao", "frio", "foco", "energia"},
	},
	// Psicologia
	{
		Name:       "jung",
		File:       "sabedoria/conhecimento/JUNG_ARCHETYPES.txt",
		Collection: "jung_archetypes",
		Tradition:  "junguiana",
		Type:       "concept",
		Tags:       []string{"arquetipo", "sombra", "individuacao", "inconsciente"},
	},
	// Ja existentes em docs/
	{
		Name:       "nasrudin",
		File:       "sabedoria/nasrudin/NASRUDIN_STORIES.txt",
		Collection: "nasrudin_stories",
		Tradition:  "sufi",
		Type:       "story",
		Tags:       []string{"humor", "paradoxo", "sabedoria", "sufi"},
	},
	{
		Name:       "esopo",
		File:       "sabedoria/esopo/FABULAS_ESOPO.txt",
		Collection: "aesop_fables",
		Tradition:  "grega",
		Type:       "fable",
		Tags:       []string{"moral", "animais", "licao", "fabula"},
	},
}

func main() {
	log.Println("🌱 EVA Wisdom Seeder - Base de Conhecimento (3072 dims)")
	log.Println("═══════════════════════════════════════════════════════════")

	// Carregar .env explicitamente (pode estar em diretorio diferente com go run)
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️ Nao encontrou .env no diretorio atual, tentando caminhos alternativos...")
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
			log.Println("⚠️ .env nao encontrado, usando variaveis de ambiente do sistema")
		}
	}

	// Debug: mostrar diretorio atual
	cwd, _ := os.Getwd()
	log.Printf("📂 Diretorio atual: %s", cwd)

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
		log.Fatalf("Failed to connect to NietzscheDB: %v", err)
	}
	defer nietzscheClient.Close()
	vectorAdapter := nietzscheInfra.NewVectorAdapter(nietzscheClient)

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
		seedAll(ctx, nietzscheClient, vectorAdapter, embedder)
	case "list":
		listSources()
	case "status":
		checkStatus(ctx, nietzscheClient)
	default:
		// Buscar fonte especifica
		found := false
		for _, ws := range WisdomSources {
			if ws.Name == source {
				seedSource(ctx, nietzscheClient, vectorAdapter, embedder, ws)
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
	fmt.Println("  list     - Lista todas as fontes disponiveis")
	fmt.Println("  status   - Verifica status das colecoes no NietzscheDB")
	fmt.Println("  <fonte>  - Seed de uma fonte especifica")
	fmt.Println("\nFontes disponiveis:")
	for _, ws := range WisdomSources {
		fmt.Printf("  %-12s → %s\n", ws.Name, ws.Collection)
	}
}

func listSources() {
	fmt.Println("\n📚 Fontes de Sabedoria Disponiveis")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("%-12s │ %-25s │ %-15s │ %s\n", "NOME", "COLECAO", "TRADICAO", "ARQUIVO")
	fmt.Println("─────────────┼───────────────────────────┼─────────────────┼────────────────────────────────")
	for _, ws := range WisdomSources {
		fmt.Printf("%-12s │ %-25s │ %-15s │ %s\n", ws.Name, ws.Collection, ws.Tradition, ws.File)
	}
}

func checkStatus(ctx context.Context, client *nietzscheInfra.Client) {
	fmt.Println("\n📊 Status das Colecoes no NietzscheDB")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("%-25s │ %s\n", "COLECAO", "STATUS")
	fmt.Println("──────────────────────────┼────────────────────────────────")

	for _, ws := range WisdomSources {
		collections, err := client.ListCollections(ctx)
		if err != nil {
			fmt.Printf("%-25s │ ❌ Erro ao listar colecoes\n", ws.Collection)
			continue
		}
		found := false
		for _, c := range collections {
			if c.Name == ws.Collection {
				fmt.Printf("%-25s │ ✅ Existe\n", ws.Collection)
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("%-25s │ ❌ Nao existe\n", ws.Collection)
		}
	}
}

func seedAll(ctx context.Context, client *nietzscheInfra.Client, vectorAdapter *nietzscheInfra.VectorAdapter, embedder *memory.EmbeddingService) {
	log.Println("🚀 Iniciando seed de TODAS as fontes...")
	log.Println("")

	for _, ws := range WisdomSources {
		seedSource(ctx, client, vectorAdapter, embedder, ws)
		log.Println("")
	}

	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("🎉 Seed completo de todas as fontes!")
}

func seedSource(ctx context.Context, client *nietzscheInfra.Client, vectorAdapter *nietzscheInfra.VectorAdapter, embedder *memory.EmbeddingService, ws WisdomSource) {
	log.Printf("📖 [%s] Processando %s...", ws.Name, ws.File)

	// Verificar se arquivo existe
	content, err := os.ReadFile(ws.File)
	if err != nil {
		log.Printf("❌ [%s] Erro ao ler arquivo: %v", ws.Name, err)
		return
	}

	// Criar colecao com dimensao correta (3072 para Gemini embeddings)
	dim := uint32(embedder.GetExpectedDimension())
	err = client.EnsureCollection(ctx, ws.Collection, dim, "cosine")
	if err != nil {
		log.Printf("⚠️ [%s] Colecao ja existe ou erro: %v", ws.Name, err)
	}

	// Parsear entradas (parser customizado por fonte)
	var entries []string
	switch ws.Name {
	case "nasrudin":
		entries = parseNasrudinStories(string(content))
	case "esopo":
		entries = parseAesopFables(string(content))
	default:
		entries = parseEntries(string(content))
	}
	log.Printf("📚 [%s] Encontradas %d entradas", ws.Name, len(entries))

	if len(entries) == 0 {
		log.Printf("⚠️ [%s] Nenhuma entrada encontrada!", ws.Name)
		return
	}

	// Processar em batches
	var batch []nietzscheInfra.BatchVectorItem
	batchSize := 10
	totalProcessed := 0

	for i, entry := range entries {
		if len(entry) < 10 {
			continue // Pular entradas muito curtas
		}

		// Gerar embedding
		textToEmbed := fmt.Sprintf("%s (%s): %s", ws.Tradition, ws.Type, entry)
		vec, err := embedder.GenerateEmbedding(ctx, textToEmbed)
		if err != nil {
			log.Printf("⚠️ [%s] Erro embedding entrada %d: %v", ws.Name, i+1, err)
			continue
		}

		payload := map[string]interface{}{
			"id":        fmt.Sprintf("%s_%d", ws.Name, i+1),
			"content":   entry,
			"source":    ws.Name,
			"tradition": ws.Tradition,
			"type":      ws.Type,
			"tags":      ws.Tags,
		}

		batch = append(batch, nietzscheInfra.BatchVectorItem{
			ID:      fmt.Sprintf("%s_%d", ws.Name, i+1),
			Vector:  vec,
			Payload: payload,
		})
		totalProcessed++

		// Upsert em batches
		if len(batch) >= batchSize {
			if err := vectorAdapter.BatchUpsert(ctx, ws.Collection, batch); err != nil {
				log.Printf("❌ [%s] Erro no upsert batch: %v", ws.Name, err)
			} else {
				log.Printf("✅ [%s] Batch: %d entradas (total: %d)", ws.Name, len(batch), totalProcessed)
			}
			batch = nil
			time.Sleep(500 * time.Millisecond) // Rate limit
		}
	}

	// Upsert restante
	if len(batch) > 0 {
		if err := vectorAdapter.BatchUpsert(ctx, ws.Collection, batch); err != nil {
			log.Printf("❌ [%s] Erro no upsert final: %v", ws.Name, err)
		} else {
			log.Printf("✅ [%s] Batch final: %d entradas", ws.Name, len(batch))
		}
	}

	log.Printf("🎉 [%s] Completo! %d entradas em '%s'", ws.Name, totalProcessed, ws.Collection)
}

// parseEntries extrai entradas numeradas de um arquivo
func parseEntries(content string) []string {
	var entries []string

	// Remove linhas de placeholder como "[... arquivo completo contem X entradas]"
	placeholderRe := regexp.MustCompile(`\[\.\.\..*?\]`)
	content = placeholderRe.ReplaceAllString(content, "")

	// Remove linhas decorativas
	decorRe := regexp.MustCompile(`[━═─]+`)
	content = decorRe.ReplaceAllString(content, "\n")

	// Dividir por linhas
	lines := strings.Split(content, "\n")

	// Regex para detectar inicio de entrada numerada
	numRe := regexp.MustCompile(`^\s*(\d+)\.\s+(.+)`)

	var currentEntry strings.Builder
	inEntry := false

	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")

		// Verifica se e nova entrada numerada
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
			// Continuar entrada atual (linha nao vazia)
			currentEntry.WriteString(" ")
			currentEntry.WriteString(line)
		}
	}

	// Salvar ultima entrada
	if inEntry && currentEntry.Len() > 0 {
		text := strings.TrimSpace(currentEntry.String())
		text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
		if len(text) >= 10 {
			entries = append(entries, text)
		}
	}

	return entries
}

// parseNasrudinStories divide prosa continua em chunks de ~350 palavras
// O arquivo e texto corrido com single newlines, sem paragrafos claros
func parseNasrudinStories(content string) []string {
	var entries []string

	// Normalizar: remover \r, colapsar whitespace
	content = strings.ReplaceAll(content, "\r", "")
	content = regexp.MustCompile(`CAP[ÍI]TULO\s*\d+`).ReplaceAllString(content, "")
	content = strings.ReplaceAll(content, "Mulla Nasrudin", "")

	// Juntar tudo numa linha, colapsar espacos
	content = regexp.MustCompile(`[\n\t]+`).ReplaceAllString(content, " ")
	content = regexp.MustCompile(`\s{2,}`).ReplaceAllString(content, " ")
	content = strings.TrimSpace(content)

	// Dividir em palavras e agrupar em chunks
	words := strings.Fields(content)
	chunkSize := 350

	for i := 0; i < len(words); i += chunkSize {
		end := i + chunkSize
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[i:end], " ")
		if len(chunk) >= 50 {
			entries = append(entries, chunk)
		}
	}

	return entries
}

// parseAesopFables divide por "Fabula" seguido de numeral romano
func parseAesopFables(content string) []string {
	var entries []string

	// Normalizar line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Split por "Fabula" + espaco + romano (case insensitive para acentos)
	// Usar split simples por "Fabula " que e mais robusto
	parts := regexp.MustCompile(`(?m)\nF[áa]bula\s+[IVXLCDM]+\s*\n`).Split(content, -1)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) < 30 {
			continue
		}

		// Remover codigos de referencia (H1865, MW1919, etc.)
		cleaned := regexp.MustCompile(`(?m)^[A-Z]+\d+\s*$`).ReplaceAllString(part, "")

		// Colapsar whitespace e newlines
		cleaned = regexp.MustCompile(`[\n\r]+`).ReplaceAllString(cleaned, " ")
		cleaned = regexp.MustCompile(`\s{2,}`).ReplaceAllString(cleaned, " ")
		cleaned = strings.TrimSpace(cleaned)

		if len(cleaned) < 30 {
			continue
		}

		entries = append(entries, cleaned)
	}

	return entries
}
