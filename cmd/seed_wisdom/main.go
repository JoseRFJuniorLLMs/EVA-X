package main

import (
	"context"
	"eva/internal/brainstem/config"
	"eva/internal/brainstem/infrastructure/vector"
	"eva/internal/hippocampus/memory"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/qdrant/go-client/qdrant"
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
		File:       "sabedoria/conhecimento/GURDJIEFF_TEACHINGS.txt",
		Collection: "gurdjieff_teachings",
		Tradition:  "quarto_caminho",
		Type:       "teaching",
		Tags:       []string{"auto-observação", "despertar", "trabalho", "consciência"},
	},
	{
		Name:       "osho",
		File:       "sabedoria/conhecimento/OSHO_INSIGHTS.txt",
		Collection: "osho_insights",
		Tradition:  "osho",
		Type:       "insight",
		Tags:       []string{"testemunho", "meditação", "celebração", "provocação"},
	},
	{
		Name:       "ouspensky",
		File:       "sabedoria/conhecimento/OUSPENSKY_FRAGMENTS.txt",
		Collection: "ouspensky_fragments",
		Tradition:  "quarto_caminho",
		Type:       "teaching",
		Tags:       []string{"máquina", "centros", "tipos", "desenvolvimento"},
	},
	{
		Name:       "nietzsche",
		File:       "sabedoria/conhecimento/NIETZSCHE_ZARATUSTRA.txt",
		Collection: "nietzsche_aphorisms",
		Tradition:  "filosofia",
		Type:       "aphorism",
		Tags:       []string{"super-homem", "vontade", "força", "transvaloração"},
	},
	// Tradições
	{
		Name:       "zen",
		File:       "sabedoria/conhecimento/ZEN_KOANS.txt",
		Collection: "zen_koans",
		Tradition:  "zen",
		Type:       "koan",
		Tags:       []string{"paradoxo", "não-mente", "iluminação", "presente"},
	},
	{
		Name:       "rumi",
		File:       "sabedoria/conhecimento/RUMI_POEMS.txt",
		Collection: "rumi_poems",
		Tradition:  "sufi",
		Type:       "poem",
		Tags:       []string{"amor", "união", "divino", "coração"},
	},
	{
		Name:       "estoicos",
		File:       "sabedoria/conhecimento/STOIC_MEDITATIONS.txt",
		Collection: "stoic_meditations",
		Tradition:  "estoica",
		Type:       "meditation",
		Tags:       []string{"aceitação", "controle", "virtude", "resiliência"},
	},
	// Técnicas
	{
		Name:       "osho_med",
		File:       "sabedoria/conhecimento/OSHO_MEDITATIONS.txt",
		Collection: "osho_meditations",
		Tradition:  "osho",
		Type:       "meditation",
		Tags:       []string{"ativa", "catarse", "energia", "silêncio"},
	},
	{
		Name:       "respiracao",
		File:       "sabedoria/conhecimento/BREATHING_SCRIPTS.txt",
		Collection: "breathing_scripts",
		Tradition:  "integrativa",
		Type:       "exercise",
		Tags:       []string{"respiração", "regulação", "calma", "energia"},
	},
	{
		Name:       "hipnose",
		File:       "sabedoria/conhecimento/SELF_HYPNOSIS_SCRIPTS.txt",
		Collection: "hypnosis_scripts",
		Tradition:  "hipnoterapia",
		Type:       "script",
		Tags:       []string{"autoindução", "relaxamento", "reprogramação", "transe"},
	},
	{
		Name:       "somatico",
		File:       "sabedoria/conhecimento/SOMATIC_EXERCISES.txt",
		Collection: "somatic_exercises",
		Tradition:  "somática",
		Type:       "exercise",
		Tags:       []string{"corpo", "grounding", "regulação", "trauma"},
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
		Tags:       []string{"respiração", "frio", "foco", "energia"},
	},
	// Psicologia
	{
		Name:       "jung",
		File:       "sabedoria/conhecimento/JUNG_ARCHETYPES.txt",
		Collection: "jung_archetypes",
		Tradition:  "junguiana",
		Type:       "concept",
		Tags:       []string{"arquétipo", "sombra", "individuação", "inconsciente"},
	},
	// Já existentes em docs/
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

	// Conectar Qdrant (usa config do .env)
	qdrantHost := cfg.QdrantHost
	qdrantPort := cfg.QdrantPort
	if qdrantHost == "" {
		qdrantHost = "localhost"
	}
	if qdrantPort == 0 {
		qdrantPort = 6333
	}
	log.Printf("🔌 Conectando ao Qdrant: %s:%d", qdrantHost, qdrantPort)

	qClient, err := vector.NewQdrantClient(qdrantHost, qdrantPort)
	if err != nil {
		log.Fatalf("❌ Erro ao conectar Qdrant: %v", err)
	}

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
		seedAll(ctx, qClient, embedder)
	case "list":
		listSources()
	case "status":
		checkStatus(ctx, qClient)
	default:
		// Buscar fonte específica
		found := false
		for _, ws := range WisdomSources {
			if ws.Name == source {
				seedSource(ctx, qClient, embedder, ws)
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
	fmt.Println("  status   - Verifica status das coleções no Qdrant")
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

func checkStatus(ctx context.Context, qClient *vector.QdrantClient) {
	fmt.Println("\n📊 Status das Coleções no Qdrant")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("%-25s │ %s\n", "COLEÇÃO", "STATUS")
	fmt.Println("──────────────────────────┼────────────────────────────────")

	for _, ws := range WisdomSources {
		info, err := qClient.GetCollectionInfo(ctx, ws.Collection)
		if err != nil {
			fmt.Printf("%-25s │ ❌ Não existe\n", ws.Collection)
		} else {
			fmt.Printf("%-25s │ ✅ %d pontos\n", ws.Collection, info.PointsCount)
		}
	}
}

func seedAll(ctx context.Context, qClient *vector.QdrantClient, embedder *memory.EmbeddingService) {
	log.Println("🚀 Iniciando seed de TODAS as fontes...")
	log.Println("")

	for _, ws := range WisdomSources {
		seedSource(ctx, qClient, embedder, ws)
		log.Println("")
	}

	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("🎉 Seed completo de todas as fontes!")
}

func seedSource(ctx context.Context, qClient *vector.QdrantClient, embedder *memory.EmbeddingService, ws WisdomSource) {
	log.Printf("📖 [%s] Processando %s...", ws.Name, ws.File)

	// Verificar se arquivo existe
	content, err := os.ReadFile(ws.File)
	if err != nil {
		log.Printf("❌ [%s] Erro ao ler arquivo: %v", ws.Name, err)
		return
	}

	// Criar coleção com dimensão correta (768 para Vertex AI)
	dim := uint64(embedder.GetExpectedDimension())
	err = qClient.CreateCollection(ctx, ws.Collection, dim)
	if err != nil {
		log.Printf("⚠️ [%s] Coleção já existe ou erro: %v", ws.Name, err)
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
	var points []*qdrant.PointStruct
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

		point := createPoint(uint64(i+1), vec, map[string]interface{}{
			"id":        fmt.Sprintf("%s_%d", ws.Name, i+1),
			"content":   entry,
			"source":    ws.Name,
			"tradition": ws.Tradition,
			"type":      ws.Type,
			"tags":      ws.Tags,
		})

		points = append(points, point)
		totalProcessed++

		// Upsert em batches
		if len(points) >= batchSize {
			if err := qClient.Upsert(ctx, ws.Collection, points); err != nil {
				log.Printf("❌ [%s] Erro no upsert batch: %v", ws.Name, err)
			} else {
				log.Printf("✅ [%s] Batch: %d entradas (total: %d)", ws.Name, len(points), totalProcessed)
			}
			points = nil
			time.Sleep(500 * time.Millisecond) // Rate limit
		}
	}

	// Upsert restante
	if len(points) > 0 {
		if err := qClient.Upsert(ctx, ws.Collection, points); err != nil {
			log.Printf("❌ [%s] Erro no upsert final: %v", ws.Name, err)
		} else {
			log.Printf("✅ [%s] Batch final: %d entradas", ws.Name, len(points))
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

// parseNasrudinStories divide prosa contínua em chunks de ~350 palavras
// O arquivo é texto corrido com single newlines, sem parágrafos claros
func parseNasrudinStories(content string) []string {
	var entries []string

	// Normalizar: remover \r, colapsar whitespace
	content = strings.ReplaceAll(content, "\r", "")
	content = regexp.MustCompile(`CAP[ÍI]TULO\s*\d+`).ReplaceAllString(content, "")
	content = strings.ReplaceAll(content, "Mulla Nasrudin", "")

	// Juntar tudo numa linha, colapsar espaços
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

// parseAesopFables divide por "Fábula" seguido de numeral romano
func parseAesopFables(content string) []string {
	var entries []string

	// Normalizar line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Split por "Fábula" + espaço + romano (case insensitive para acentos)
	// Usar split simples por "Fábula " que é mais robusto
	parts := regexp.MustCompile(`(?m)\nF[áa]bula\s+[IVXLCDM]+\s*\n`).Split(content, -1)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) < 30 {
			continue
		}

		// Remover códigos de referência (H1865, MW1919, etc.)
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

func createPoint(id uint64, vec []float32, payload map[string]interface{}) *qdrant.PointStruct {
	point := &qdrant.PointStruct{
		Id: &qdrant.PointId{
			PointIdOptions: &qdrant.PointId_Num{Num: id},
		},
		Vectors: &qdrant.Vectors{
			VectorsOptions: &qdrant.Vectors_Vector{Vector: &qdrant.Vector{Data: vec}},
		},
		Payload: make(map[string]*qdrant.Value),
	}

	for k, v := range payload {
		point.Payload[k] = toQdrantValue(v)
	}

	return point
}

func toQdrantValue(v interface{}) *qdrant.Value {
	switch val := v.(type) {
	case string:
		return &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: val}}
	case int:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}
	case int64:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: val}}
	case float64:
		return &qdrant.Value{Kind: &qdrant.Value_DoubleValue{DoubleValue: val}}
	case []string:
		list := &qdrant.ListValue{}
		for _, s := range val {
			list.Values = append(list.Values, &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: s}})
		}
		return &qdrant.Value{Kind: &qdrant.Value_ListValue{ListValue: list}}
	default:
		return &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: fmt.Sprintf("%v", val)}}
	}
}
