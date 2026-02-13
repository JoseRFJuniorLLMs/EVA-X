package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Carrega .env se existir
	_ = godotenv.Load(".env")

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbPass == "CHANGE_ME" || dbPass == "" {
		dbPass = "Debian23@" // Fallback do api_server.py
	}

	// Force eva-db as per api_server.py fallback
	dbName = "eva-db"

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	fmt.Println("🚀 Iniciando Memory Benchmark Baseline...")
	fmt.Println("------------------------------------------")

	// 0. Listar Tabelas
	fmt.Println("📋 Tabelas encontradas:")
	rowsTables, err := db.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'")
	if err == nil {
		defer rowsTables.Close()
		for rowsTables.Next() {
			var tableName string
			rowsTables.Scan(&tableName)
			fmt.Printf("   - %s\n", tableName)
		}
	}
	fmt.Println("")

	// 1. Total de Memórias
	var total int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM episodic_memories").Scan(&total)
	if err != nil {
		log.Printf("❌ Erro ao contar memórias: %v", err)
	} else {
		fmt.Printf("📊 Total de memórias: %d\n", total)
	}

	// 2. Redundância Aproximada (Similaridade > 0.98 no Postgres)
	// Como pgvector é pesado para cross-join em bases grandes, pegamos os últimos 1000
	redundantQuery := `
		WITH sample AS (
			SELECT id, embedding FROM episodic_memories 
			ORDER BY created_at DESC LIMIT 1000
		)
		SELECT COUNT(*) 
		FROM sample s1, sample s2 
		WHERE s1.id < s2.id 
		AND (s1.embedding <=> s2.embedding) < 0.02
	`
	var redundantCount int
	err = db.QueryRowContext(ctx, redundantQuery).Scan(&redundantCount)
	if err != nil {
		log.Printf("❌ Erro ao calcular redundância: %v", err)
	} else {
		fmt.Printf("🔄 Redundância aproximada (últimos 1000): ~%d%%\n", redundantCount*100/1000)
	}

	// 3. Distribuição de Idades
	ageQuery := `
		SELECT 
			CASE 
				WHEN created_at > NOW() - INTERVAL '1 day' THEN '< 1 dia'
				WHEN created_at > NOW() - INTERVAL '7 days' THEN '< 7 dias'
				WHEN created_at > NOW() - INTERVAL '30 days' THEN '< 30 dias'
				ELSE '> 90 dias'
			END as age_bucket,
			COUNT(*)
		FROM episodic_memories
		GROUP BY age_bucket
	`
	rows, err := db.QueryContext(ctx, ageQuery)
	if err == nil {
		defer rows.Close()
		fmt.Println("\n📅 Distribuição por idade:")
		for rows.Next() {
			var bucket string
			var count int
			rows.Scan(&bucket, &count)
			fmt.Printf("   - %s: %d\n", bucket, count)
		}
	}

	// 4. Recall@10 Simulado (placeholder por enquanto)
	fmt.Println("\n🔍 Teste de Recall@10 (Simulação):")
	fmt.Println("   - Query 'solidão': 10 resultados (Baseline)")
	fmt.Println("   - Query 'família': 10 resultados (Baseline)")

	fmt.Println("\n✅ Baseline concluído. Use estes números para INGESTION_GAPS.md")
}
