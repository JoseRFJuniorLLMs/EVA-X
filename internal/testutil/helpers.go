package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestDB fornece uma conexão de banco de dados para testes
type TestDB struct {
	Conn *sql.DB
}

// SetupTestDB cria uma conexão de teste com o banco
// Usa variável de ambiente TEST_DATABASE_URL ou fallback para local
func SetupTestDB(t *testing.T) *TestDB {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		// Fallback para banco local de teste
		dbURL = "postgres://postgres:postgres@localhost:5432/eva_test?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("Falha ao conectar ao banco de teste: %v", err)
	}

	// Verificar conexão
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Banco de teste não disponível: %v", err)
	}

	return &TestDB{Conn: db}
}

// Cleanup fecha a conexão e limpa dados de teste
func (tdb *TestDB) Cleanup(t *testing.T) {
	t.Helper()
	if tdb.Conn != nil {
		tdb.Conn.Close()
	}
}

// TruncateTable limpa uma tabela específica
func (tdb *TestDB) TruncateTable(t *testing.T, tableName string) {
	t.Helper()
	_, err := tdb.Conn.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", tableName))
	if err != nil {
		t.Logf("Aviso: não foi possível truncar tabela %s: %v", tableName, err)
	}
}

// InsertTestIdoso cria um idoso de teste e retorna o ID
func (tdb *TestDB) InsertTestIdoso(t *testing.T, nome, cpf string) int64 {
	t.Helper()

	var id int64
	err := tdb.Conn.QueryRow(`
		INSERT INTO idosos (nome, cpf, email, telefone)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (cpf) DO UPDATE SET nome = EXCLUDED.nome
		RETURNING id
	`, nome, cpf, "test@test.com", "11999999999").Scan(&id)

	if err != nil {
		t.Fatalf("Falha ao criar idoso de teste: %v", err)
	}

	return id
}

// CleanupTestIdoso remove o idoso de teste
func (tdb *TestDB) CleanupTestIdoso(t *testing.T, idosoID int64) {
	t.Helper()
	tdb.Conn.Exec("DELETE FROM idosos WHERE id = $1", idosoID)
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
