package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"eva/internal/brainstem/database"
	"eva/pkg/crypto"

	nietzsche "nietzsche-sdk"
)

func main() {
	// Connect to NietzscheDB gRPC
	sdk, err := nietzsche.ConnectInsecure("localhost:50051")
	if err != nil {
		panic(fmt.Sprintf("failed to connect to NietzscheDB: %v", err))
	}
	defer sdk.Close()

	db := database.NewNietzscheDB(sdk)
	ctx := context.Background()

	// Patient data (from env vars, with fallbacks)
	cpf := os.Getenv("CREATOR_CPF")
	if cpf == "" {
		cpf = "64525430249"
	}
	nome := os.Getenv("CREATOR_NAME")
	if nome == "" {
		nome = "Jose R F Junior"
	}

	content := map[string]interface{}{
		"nome":                        crypto.Encrypt(nome),
		"cpf":                         crypto.Encrypt(cpf),
		"cpf_hash":                    crypto.HashCPF(cpf),
		"telefone":                    "",
		"email":                       "web2ajax@gmail.com",
		"data_nascimento":             "1985-01-01T00:00:00Z",
		"ativo":                       true,
		"nivel_cognitivo":             "normal",
		"limitacoes_auditivas":        false,
		"usa_aparelho_auditivo":       false,
		"tom_voz":                     "calmo",
		"preferencia_horario_ligacao": "manha",
		"timezone":                    "America/Sao_Paulo",
		"created_at":                  time.Now().Format(time.RFC3339),
	}

	id, err := db.Insert(ctx, "idosos", content)
	if err != nil {
		panic(fmt.Sprintf("failed to insert idoso: %v", err))
	}

	fmt.Printf("Idoso criado com sucesso!\n")
	fmt.Printf("  ID:       %d\n", id)
	fmt.Printf("  Nome:     %s\n", nome)
	fmt.Printf("  CPF:      %s\n", cpf)
	fmt.Printf("  CPF Hash: %s\n", crypto.HashCPF(cpf))

	// Verify it can be found
	found, findErr := db.GetIdosoByCPF(cpf)
	if findErr != nil {
		fmt.Printf("  WARN: Verificacao falhou: %v\n", findErr)
	} else if found != nil {
		fmt.Printf("  Verificado: encontrado ID=%d Nome=%s\n", found.ID, found.Nome)
	}
}
