package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("ERRO conectar:", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Verificar quem tem coleções
	var count int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM idosos WHERE colecoes != '' AND colecoes IS NOT NULL").Scan(&count)
	fmt.Printf("📊 %d idoso(s) com coleções atribuídas:\n", count)

	// Mostrar quem tem coleções
	rows, _ := db.QueryContext(ctx, "SELECT id, nome, colecoes FROM idosos WHERE colecoes != '' AND colecoes IS NOT NULL")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			var nome, col string
			rows.Scan(&id, &nome, &col)
			fmt.Printf("  📚 Idoso #%d (%s): %s\n", id, nome, col)
		}
	}
}
