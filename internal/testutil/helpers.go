// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// TestDB fornece uma conexão de banco de dados para testes
type TestDB struct {
	DB *database.DB
}

// SetupTestDB cria uma conexão de teste com NietzscheDB
// Usa variável de ambiente TEST_NIETZSCHE_URL ou fallback para local
func SetupTestDB(t *testing.T) *TestDB {
	t.Helper()

	addr := os.Getenv("TEST_NIETZSCHE_URL")
	if addr == "" {
		addr = "localhost:50051"
	}

	client, err := nietzscheInfra.NewClient(addr)
	if err != nil {
		t.Skipf("NietzscheDB de teste não disponível: %v", err)
	}

	db := database.NewNietzscheDB(client.SDK())

	return &TestDB{DB: db}
}

// Cleanup fecha a conexão e limpa dados de teste
func (tdb *TestDB) Cleanup(t *testing.T) {
	t.Helper()
	// NietzscheDB client cleanup if needed
}

// TruncateTable is a no-op for NietzscheDB (no TRUNCATE support)
func (tdb *TestDB) TruncateTable(t *testing.T, tableName string) {
	t.Helper()
	t.Logf("TruncateTable: NietzscheDB does not support TRUNCATE, skipping %s", tableName)
}

// InsertTestIdoso cria um idoso de teste e retorna o ID
func (tdb *TestDB) InsertTestIdoso(t *testing.T, nome, cpf string) int64 {
	t.Helper()

	ctx := context.Background()
	id, err := tdb.DB.Insert(ctx, "idosos", map[string]interface{}{
		"nome":       nome,
		"cpf":        cpf,
		"email":      "test@test.com",
		"telefone":   "11999999999",
		"ativo":      true,
		"created_at": time.Now().Format(time.RFC3339),
	})

	if err != nil {
		t.Fatalf("Falha ao criar idoso de teste: %v", err)
	}

	return id
}

// CleanupTestIdoso remove o idoso de teste (note: NietzscheDB delete not always available)
func (tdb *TestDB) CleanupTestIdoso(t *testing.T, idosoID int64) {
	t.Helper()
	t.Logf("CleanupTestIdoso: skipping delete for idoso %d (NietzscheDB)", idosoID)
}

// TestContext cria um contexto com timeout para testes
func TestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// AssertNoError falha o teste se houver erro
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// AssertError falha o teste se NÃO houver erro
func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: esperava erro, mas não houve", msg)
	}
}

// AssertEqual compara dois valores
func AssertEqual[T comparable](t *testing.T, expected, actual T, msg string) {
	t.Helper()
	if expected != actual {
		t.Fatalf("%s: esperado %v, obteve %v", msg, expected, actual)
	}
}

// AssertNotNil falha se o valor for nil
func AssertNotNil(t *testing.T, value interface{}, msg string) {
	t.Helper()
	if value == nil {
		t.Fatalf("%s: valor não deveria ser nil", msg)
	}
}

// AssertTrue falha se a condição for falsa
func AssertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Fatalf("%s: condição deveria ser true", msg)
	}
}

// AssertFalse falha se a condição for verdadeira
func AssertFalse(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Fatalf("%s: condição deveria ser false", msg)
	}
}

// AssertInRange verifica se um valor está dentro de um range
func AssertInRange(t *testing.T, value, min, max int, msg string) {
	t.Helper()
	if value < min || value > max {
		t.Fatalf("%s: valor %d deveria estar entre %d e %d", msg, value, min, max)
	}
}

// Ensure fmt is used (for backward compatibility)
var _ = fmt.Sprintf
