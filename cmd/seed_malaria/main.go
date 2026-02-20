package main

import (
	"context"
	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/brainstem/infrastructure/vector"
	"eva-mind/internal/hippocampus/memory"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/qdrant/go-client/qdrant"
)

// MalariaSource define uma fonte de conhecimento sobre malária
type MalariaSource struct {
	Name       string
	File       string
	Collection string
	Category   string
	Tags       []string
}

// Fontes de conhecimento sobre malária
var MalariaSources = []MalariaSource{
	{
		Name:       "global",
		File:       "sabedoria/conhecimento/MALARIA_GLOBAL.txt",
		Collection: "malaria_global",
		Category:   "epidemiologia_global",
		Tags:       []string{"malária", "epidemiologia", "tratamento", "prevenção", "vacinas", "OMS", "global"},
	},
	{
		Name:       "africa",
		File:       "sabedoria/conhecimento/MALARIA_AFRICA.txt",
		Collection: "malaria_africa",
		Category:   "malária_africa",
		Tags:       []string{"malária", "áfrica", "subsaariana", "epidemiologia", "vacinas", "ITN", "SMC"},
	},
	{
		Name:       "angola",
		File:       "sabedoria/conhecimento/MALARIA_ANGOLA.txt",
		Collection: "malaria_angola",
		Category:   "malária_angola",
		Tags:       []string{"malária", "angola", "luanda", "províncias", "PNCM", "PMI", "Global Fund"},
	},
}

func main() {
	log.Println("🦟 EVA Malaria Knowledge Seeder")
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("⚠️  ACESSO RESTRITO: Estas coleções são filtradas por CPF")
	log.Println("═══════════════════════════════════════════════════════════")

	// Carregar .env
	if err := godotenv.Load(); err != nil {
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

	cwd, _ := os.Getwd()
	log.Printf("📂 Diretório atual: %s", cwd)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Config error: %v", err)
	}

	// Verificar credenciais
	vertexToken := os.Getenv("VERTEX_ACCESS_TOKEN")
	if vertexToken != "" {
		log.Printf("🔐 Vertex AI Token: %s...%s (len=%d)", vertexToken[:8], vertexToken[len(vertexToken)-4:], len(vertexToken))
	} else if cfg.GoogleAPIKey != "" {
		log.Printf("🔑 API Key: %s...%s (len=%d)", cfg.GoogleAPIKey[:8], cfg.GoogleAPIKey[len(cfg.GoogleAPIKey)-4:], len(cfg.GoogleAPIKey))
	} else {
		log.Fatal("❌ Nenhuma credencial encontrada (VERTEX_ACCESS_TOKEN ou GOOGLE_API_KEY)")
	}

	// Conectar Qdrant
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
		found := false
		for _, ms := range MalariaSources {
			if ms.Name == source {
				seedSource(ctx, qClient, embedder, ms)
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
	fmt.Println("\n🦟 EVA Malaria Knowledge Seeder")
	fmt.Println("════════════════════════════════")
	fmt.Println("\nUso: seed_malaria <comando>")
	fmt.Println("\nComandos:")
	fmt.Println("  all      - Seed de todas as fontes de malária")
	fmt.Println("  list     - Lista todas as fontes disponíveis")
	fmt.Println("  status   - Verifica status das coleções no Qdrant")
	fmt.Println("  <fonte>  - Seed de uma fonte específica")
	fmt.Println("\nFontes disponíveis:")
	for _, ms := range MalariaSources {
		fmt.Printf("  %-12s → %s\n", ms.Name, ms.Collection)
	}
	fmt.Println("\n⚠️  NOTA: Estas coleções só são acessíveis via CPF autorizado")
}

func listSources() {
	fmt.Println("\n🦟 Fontes de Conhecimento sobre Malária")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("%-12s │ %-20s │ %-25s │ %s\n", "NOME", "COLEÇÃO", "CATEGORIA", "ARQUIVO")
	fmt.Println("─────────────┼──────────────────────┼───────────────────────────┼─────────────────────────────────────")
	for _, ms := range MalariaSources {
		fmt.Printf("%-12s │ %-20s │ %-25s │ %s\n", ms.Name, ms.Collection, ms.Category, ms.File)
	}
}

func checkStatus(ctx context.Context, qClient *vector.QdrantClient) {
	fmt.Println("\n📊 Status das Coleções de Malária no Qdrant")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("%-20s │ %s\n", "COLEÇÃO", "STATUS")
	fmt.Println("─────────────────────┼────────────────────────────────")

	for _, ms := range MalariaSources {
		info, err := qClient.GetCollectionInfo(ctx, ms.Collection)
		if err != nil {
			fmt.Printf("%-20s │ ❌ Não existe\n", ms.Collection)
		} else {
			fmt.Printf("%-20s │ ✅ %d pontos\n", ms.Collection, info.PointsCount)
		}
	}
}

func seedAll(ctx context.Context, qClient *vector.QdrantClient, embedder *memory.EmbeddingService) {
	log.Println("🚀 Iniciando seed de TODAS as fontes de malária...")
	log.Println("")

	totalEntries := 0
	for _, ms := range MalariaSources {
		n := seedSource(ctx, qClient, embedder, ms)
		totalEntries += n
		log.Println("")
	}

	log.Println("═══════════════════════════════════════════════════════════")
	log.Printf("🎉 Seed completo! Total: %d entradas em %d coleções", totalEntries, len(MalariaSources))
}

func seedSource(ctx context.Context, qClient *vector.QdrantClient, embedder *memory.EmbeddingService, ms MalariaSource) int {
	log.Printf("🦟 [%s] Processando %s...", ms.Name, ms.File)

	content, err := os.ReadFile(ms.File)
	if err != nil {
		log.Printf("❌ [%s] Erro ao ler arquivo: %v", ms.Name, err)
		return 0
	}

	// Criar coleção
	dim := uint64(embedder.GetExpectedDimension())
	err = qClient.CreateCollection(ctx, ms.Collection, dim)
	if err != nil {
		log.Printf("⚠️ [%s] Coleção já existe ou erro: %v", ms.Name, err)
	}

	// Parsear entradas numeradas
	entries := parseNumberedEntries(string(content))
	log.Printf("📚 [%s] Encontradas %d entradas", ms.Name, len(entries))

	if len(entries) == 0 {
		log.Printf("⚠️ [%s] Nenhuma entrada encontrada!", ms.Name)
		return 0
	}

	// Processar em batches
	var points []*qdrant.PointStruct
	batchSize := 5 // Menor batch por entrada grande
	totalProcessed := 0

	for i, entry := range entries {
		if len(entry.content) < 20 {
			continue
		}

		// Gerar embedding com contexto
		textToEmbed := fmt.Sprintf("malária %s: %s", ms.Category, entry.content)
		vec, err := embedder.GenerateEmbedding(ctx, textToEmbed)
		if err != nil {
			log.Printf("⚠️ [%s] Erro embedding entrada %d: %v", ms.Name, i+1, err)
			time.Sleep(1 * time.Second) // Rate limit
			continue
		}

		point := createPoint(uint64(i+1), vec, map[string]interface{}{
			"id":       fmt.Sprintf("malaria_%s_%d", ms.Name, i+1),
			"title":    entry.title,
			"content":  entry.content,
			"source":   fmt.Sprintf("malaria_%s", ms.Name),
			"category": ms.Category,
			"type":     "knowledge",
			"tags":     ms.Tags,
		})

		points = append(points, point)
		totalProcessed++

		// Upsert em batches
		if len(points) >= batchSize {
			if err := qClient.Upsert(ctx, ms.Collection, points); err != nil {
				log.Printf("❌ [%s] Erro no upsert batch: %v", ms.Name, err)
			} else {
				log.Printf("✅ [%s] Batch: %d entradas (total: %d/%d)", ms.Name, len(points), totalProcessed, len(entries))
			}
			points = nil
			time.Sleep(500 * time.Millisecond) // Rate limit
		}
	}

	// Upsert restante
	if len(points) > 0 {
		if err := qClient.Upsert(ctx, ms.Collection, points); err != nil {
			log.Printf("❌ [%s] Erro no upsert final: %v", ms.Name, err)
		} else {
			log.Printf("✅ [%s] Batch final: %d entradas", ms.Name, len(points))
		}
	}

	log.Printf("🎉 [%s] Completo! %d entradas em '%s'", ms.Name, totalProcessed, ms.Collection)
	return totalProcessed
}

type parsedEntry struct {
	title   string
	content string
}

// parseNumberedEntries extrai entradas numeradas com título
func parseNumberedEntries(content string) []parsedEntry {
	var entries []parsedEntry

	lines := strings.Split(content, "\n")
	numRe := regexp.MustCompile(`^\s*(\d+)\.\s+([A-ZÁÉÍÓÚÂÊÔÃÕÇÜ\s\-\/]+):\s*(.+)`)

	var currentEntry *parsedEntry

	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")

		if match := numRe.FindStringSubmatch(line); match != nil {
			// Salvar entrada anterior
			if currentEntry != nil && len(currentEntry.content) >= 20 {
				entries = append(entries, *currentEntry)
			}

			// Nova entrada
			title := strings.TrimSpace(match[2])
			content := strings.TrimSpace(match[3])
			currentEntry = &parsedEntry{
				title:   title,
				content: content,
			}
		} else if currentEntry != nil && len(line) > 0 {
			currentEntry.content += " " + strings.TrimSpace(line)
		}
	}

	// Última entrada
	if currentEntry != nil && len(currentEntry.content) >= 20 {
		entries = append(entries, *currentEntry)
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
