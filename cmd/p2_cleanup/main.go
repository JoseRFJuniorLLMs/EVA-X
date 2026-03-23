package main

import (
	"context"
	"fmt"
	"log"
	"os"

	nietzsche "nietzsche-sdk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	client, err := nietzsche.Connect("34.56.82.116:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	ctx := context.Background()

	fmt.Println("=== P2-A: Cleaning eva_learnings ===")
	
	// Query to get nodes
	qRes, err := client.Query(ctx, "MATCH (n) RETURN n LIMIT 500", nil, "eva_learnings")
	if err != nil {
		fmt.Printf("ERR_QUERY: %v\n", err)
	} else if qRes == nil {
		fmt.Printf("ERR_RES_NIL\n")
	} else {
		fmt.Printf("NODES_FOUND: %d\n", len(qRes.Nodes))
		deleted := 0
		for _, node := range qRes.Nodes {
			err = client.DeleteNode(ctx, node.ID, "eva_learnings")
			if err == nil {
				deleted++
			} else {
				fmt.Printf("ERR_DEL %s: %v\n", node.ID, err)
			}
		}
		fmt.Printf("NODES_DELETED: %d\n", deleted)
	}

	os.Exit(0)
}
