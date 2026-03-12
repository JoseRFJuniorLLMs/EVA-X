// Temporary tool to create the creator's Idoso record in NietzscheDB
// Usage: ENCRYPTION_KEY=... NIETZSCHE_GRPC_ADDR=136.111.0.47:50051 go run ./cmd/create_idoso
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"eva/pkg/crypto"

	nietzsche "nietzsche-sdk"
)

func main() {
	addr := os.Getenv("NIETZSCHE_GRPC_ADDR")
	if addr == "" {
		addr = "136.111.0.47:50051"
	}

	// Connect to NietzscheDB
	client, err := nietzsche.ConnectInsecure(addr)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Creator data
	nome := "Jose R F Junior"
	cpf := "64525430249"
	now := time.Now().Format(time.RFC3339)

	// Encrypt fields
	encNome := crypto.Encrypt(nome)
	encCPF := crypto.Encrypt(cpf)
	cpfHash := crypto.HashCPF(cpf)

	fmt.Printf("Nome: %s -> %s\n", nome, encNome)
	fmt.Printf("CPF:  %s -> hash: %s\n", cpf, cpfHash)

	content := map[string]interface{}{
		"node_label":       "idosos",
		"id":               float64(1),
		"nome":             encNome,
		"cpf":              encCPF,
		"cpf_hash":         cpfHash,
		"telefone":         "",
		"ativo":            true,
		"nivel_cognitivo":  "super_genio",
		"tom_voz":          "padrao",
		"idioma":           "pt-BR",
		"persona_preferida": "psychologist",
		"criado_em":        now,
		"atualizado_em":    now,
	}

	// Use MergeNode to avoid duplicates
	result, err := client.MergeNode(ctx, nietzsche.MergeNodeOpts{
		Collection: "eva_mind",
		NodeType:   "Semantic",
		MatchKeys: map[string]interface{}{
			"node_label": "idosos",
			"cpf_hash":   cpfHash,
		},
		OnCreateSet: content,
		OnMatchSet: map[string]interface{}{
			"nome":           encNome,
			"ativo":          true,
			"atualizado_em":  now,
		},
	})
	if err != nil {
		log.Fatalf("MergeNode failed: %v", err)
	}

	fmt.Printf("✅ Idoso criado/atualizado: NodeID=%s, Created=%v\n", result.NodeID, result.Created)
}
