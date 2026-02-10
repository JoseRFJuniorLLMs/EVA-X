package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

// Hardcoded for audit purposes as per previous context
const DB_URL = "postgres://postgres:Debian23%40@104.248.219.200:5432/eva-db?sslmode=disable"

func main() {
	auditFunction("search_similar_memories")
}

func auditFunction(funcName string) {
	fmt.Printf("🔍 Auditing function: %s\n", funcName)

	db, err := sql.Open("postgres", DB_URL)
	if err != nil {
		log.Fatalf("❌ Error opening DB: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Error connecting to DB: %v", err)
	}

	query := fmt.Sprintf(`
        SELECT 
            t.typname,
            att.attname,
            t.oid::regtype
        FROM pg_proc p
        JOIN pg_namespace n ON p.pronamespace = n.oid
        LEFT JOIN pg_type t ON p.prorettype = t.oid
        LEFT JOIN pg_attribute att ON t.typrelid = att.attrelid
        WHERE p.proname = '%s'
        ORDER BY att.attnum;
    `, funcName)

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("❌ Query error: %v", err)
	}
	defer rows.Close()

	// found := false
	i := 1
	for rows.Next() {
		// found = true
		var typeName string
		var attName, typeID sql.NullString
		if err := rows.Scan(&typeName, &attName, &typeID); err != nil {
			log.Printf("⚠️ Scan error: %v", err)
			continue
		}
		if i == 1 {
			fmt.Printf("✅ Function found. Return Type: %s\n", typeName)
			fmt.Println("📋 Columns:")
		}
		fmt.Printf("  %d. Name: %s | Type: %s\n", i, attName.String, typeID.String)
		i++
	}

	fmt.Println("📜 Fetching Source Code...")
	var src string
	err2 := db.QueryRow(fmt.Sprintf("SELECT prosrc FROM pg_proc WHERE proname = '%s'", funcName)).Scan(&src)
	if err2 == nil {
		fmt.Println("-------------------------------------------")
		fmt.Println(src)
		fmt.Println("-------------------------------------------")
	} else {
		log.Printf("❌ Could not fetch source: %v", err)
	}
}
