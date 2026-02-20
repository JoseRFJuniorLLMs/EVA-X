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

// MalariaSource define uma fonte de conhecimento sobre malaria
type MalariaSource struct {
	Name       string
	File       string
	Collection string
	Category   string
	Tags       []string
}

// Fontes de conhecimento sobre malaria
var MalariaSources = []MalariaSource{
	{
		Name:       "global",
		File:       "sabedoria/conhecimento/MALARIA_GLOBAL.txt",
		Collection: "malaria_global",
		Category:   "epidemiologia_global",
		Tags:       []string{"malaria", "epidemiologia", "tratamento", "prevencao", "vacinas", "OMS", "global"},
	},
	{
		Name:       "africa",
		File:       "sabedoria/conhecimento/MALARIA_AFRICA.txt",
		Collection: "malaria_africa",
		Category:   "malaria_africa",
		Tags:       []string{"malaria", "africa", "subsaariana", "epidemiologia", "vacinas", "ITN", "SMC"},
	},
	{
		Name:       "angola",
		File:       "sabedoria/conhecimento/MALARIA_ANGOLA.txt",
		Collection: "malaria_angola",
		Category:   "malaria_angola",
		Tags:       []string{"malaria", "angola", "luanda", "provincias", "PNCM", "PMI", "Global Fund"},
	},
}

func main() {
	log.Println("🦟 EVA Malaria Knowledge Seeder")
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("⚠️  ACESSO RESTRITO: Estas colecoes sao filtradas por CPF")
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
			log.Println("⚠️ .env nao encontrado, usando variaveis de ambiente do sistema")
		}
	}

	cwd, _ := os.Getwd()
	log.Printf("📂 Diretorio atual: %s", cwd)

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
		found := false
		for _, ms := range MalariaSources {
			if ms.Name == source {
				seedSource(ctx, nietzscheClient, vectorAdapter, embedder, ms)
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
	fmt.Println("  all      - Seed de todas as fontes de malaria")
	fmt.Println("  list     - Lista todas as fontes disponiveis")
	fmt.Println("  status   - Verifica status das colecoes no NietzscheDB")
	fmt.Println("  <fonte>  - Seed de uma fonte especifica")
	fmt.Println("\nFontes disponiveis:")
	for _, ms := range MalariaSources {
		fmt.Printf("  %-12s → %s\n", ms.Name, ms.Collection)
	}
	fmt.Println("\n⚠️  NOTA: Estas colecoes so sao acessiveis via CPF autorizado")
}

func listSources() {
	fmt.Println("\n🦟 Fontes de Conhecimento sobre Malaria")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("%-12s │ %-20s │ %-25s │ %s\n", "NOME", "COLECAO", "CATEGORIA", "ARQUIVO")
	fmt.Println("─────────────┼──────────────────────┼───────────────────────────┼─────────────────────────────────────")
	for _, ms := range MalariaSources {
		fmt.Printf("%-12s │ %-20s │ %-25s │ %s\n", ms.Name, ms.Collection, ms.Category, ms.File)
	}
}

func checkStatus(ctx context.Context, client *nietzscheInfra.Client) {
	fmt.Println("\n📊 Status das Colecoes de Malaria no NietzscheDB")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("%-20s │ %s\n", "COLECAO", "STATUS")
	fmt.Println("─────────────────────┼────────────────────────────────")

	for _, ms := range MalariaSources {
		collections, err := client.ListCollections(ctx)
		if err != nil {
			fmt.Printf("%-20s │ ❌ Erro ao listar colecoes\n", ms.Collection)
			continue
		}
		found := false
		for _, c := range collections {
			if c.Name == ms.Collection {
				fmt.Printf("%-20s │ ✅ Existe\n", ms.Collection)
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("%-20s │ ❌ Nao existe\n", ms.Collection)
		}
	}
}

func seedAll(ctx context.Context, client *nietzscheInfra.Client, vectorAdapter *nietzscheInfra.VectorAdapter, embedder *memory.EmbeddingService) {
	log.Println("🚀 Iniciando seed de TODAS as fontes de malaria...")
	log.Println("")

	totalEntries := 0
	for _, ms := range MalariaSources {
		n := seedSource(ctx, client, vectorAdapter, embedder, ms)
		totalEntries += n
		log.Println("")
	}

	log.Println("═══════════════════════════════════════════════════════════")
	log.Printf("🎉 Seed completo! Total: %d entradas em %d colecoes", totalEntries, len(MalariaSources))
}

func seedSource(ctx context.Context, client *nietzscheInfra.Client, vectorAdapter *nietzscheInfra.VectorAdapter, embedder *memory.EmbeddingService, ms MalariaSource) int {
	log.Printf("🦟 [%s] Processando %s...", ms.Name, ms.File)

	content, err := os.ReadFile(ms.File)
	if err != nil {
		log.Printf("❌ [%s] Erro ao ler arquivo: %v", ms.Name, err)
		return 0
	}

	// Criar colecao
	dim := uint32(embedder.GetExpectedDimension())
	err = client.EnsureCollection(ctx, ms.Collection, dim, "cosine")
	if err != nil {
		log.Printf("⚠️ [%s] Colecao ja existe ou erro: %v", ms.Name, err)
	}

	// Parsear entradas numeradas
	entries := parseNumberedEntries(string(content))
	log.Printf("📚 [%s] Encontradas %d entradas", ms.Name, len(entries))

	if len(entries) == 0 {
		log.Printf("⚠️ [%s] Nenhuma entrada encontrada!", ms.Name)
		return 0
	}

	// Processar em batches
	var batch []nietzscheInfra.BatchVectorItem
	batchSize := 5 // Menor batch por entrada grande
	totalProcessed := 0

	for i, entry := range entries {
		if len(entry.content) < 20 {
			continue
		}

		// Gerar embedding com contexto
		textToEmbed := fmt.Sprintf("malaria %s: %s", ms.Category, entry.content)
		vec, err := embedder.GenerateEmbedding(ctx, textToEmbed)
		if err != nil {
			log.Printf("⚠️ [%s] Erro embedding entrada %d: %v", ms.Name, i+1, err)
			time.Sleep(1 * time.Second) // Rate limit
			continue
		}

		payload := map[string]interface{}{
			"id":       fmt.Sprintf("malaria_%s_%d", ms.Name, i+1),
			"title":    entry.title,
			"content":  entry.content,
			"source":   fmt.Sprintf("malaria_%s", ms.Name),
			"category": ms.Category,
			"type":     "knowledge",
			"tags":     ms.Tags,
		}

		batch = append(batch, nietzscheInfra.BatchVectorItem{
			ID:      fmt.Sprintf("malaria_%s_%d", ms.Name, i+1),
			Vector:  vec,
			Payload: payload,
		})
		totalProcessed++

		// Upsert em batches
		if len(batch) >= batchSize {
			if err := vectorAdapter.BatchUpsert(ctx, ms.Collection, batch); err != nil {
				log.Printf("❌ [%s] Erro no upsert batch: %v", ms.Name, err)
			} else {
				log.Printf("✅ [%s] Batch: %d entradas (total: %d/%d)", ms.Name, len(batch), totalProcessed, len(entries))
			}
			batch = nil
			time.Sleep(500 * time.Millisecond) // Rate limit
		}
	}

	// Upsert restante
	if len(batch) > 0 {
		if err := vectorAdapter.BatchUpsert(ctx, ms.Collection, batch); err != nil {
			log.Printf("❌ [%s] Erro no upsert final: %v", ms.Name, err)
		} else {
			log.Printf("✅ [%s] Batch final: %d entradas", ms.Name, len(batch))
		}
	}

	log.Printf("🎉 [%s] Completo! %d entradas em '%s'", ms.Name, totalProcessed, ms.Collection)
	return totalProcessed
}

type parsedEntry struct {
	title   string
	content string
}

// parseNumberedEntries extrai entradas numeradas com titulo
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

	// Ultima entrada
	if currentEntry != nil && len(currentEntry.content) >= 20 {
		entries = append(entries, *currentEntry)
	}

	return entries
}
