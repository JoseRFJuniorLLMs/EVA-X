package swarm

import (
	"fmt"
	"log"
)

// SetupAllSwarms registra todos os swarm agents no orchestrator
// Esta função é o ponto central de bootstrap do sistema de swarms
func SetupAllSwarms(orchestrator *Orchestrator, agents ...SwarmAgent) error {
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🐝 EVA-Mind Swarm Initialization")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, agent := range agents {
		if err := orchestrator.Register(agent); err != nil {
			return fmt.Errorf("falha ao registrar swarm '%s': %w", agent.Name(), err)
		}
	}

	stats := orchestrator.Stats()
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("✅ Swarm System Ready")
	log.Printf("  Swarms: %v", stats["swarm_count"])
	log.Printf("  Tools:  %v", stats["tool_count"])
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return nil
}
