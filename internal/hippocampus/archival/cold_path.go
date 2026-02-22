// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package archival

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"eva/internal/brainstem/infrastructure/storage"
	"eva/internal/hippocampus/memory"
)

// ColdPathService orquestra o arquivamento de memórias distais
type ColdPathService struct {
	store       *memory.MemoryStore
	blobAdapter *storage.BlobAdapter
}

// NewColdPathService cria um novo serviço de arquivamento
func NewColdPathService(store *memory.MemoryStore, blobAdapter *storage.BlobAdapter) *ColdPathService {
	return &ColdPathService{
		store:       store,
		blobAdapter: blobAdapter,
	}
}

// ArchiveOldMemories move memórias antigas e de baixa importância para o S3 (Cold Path)
func (s *ColdPathService) ArchiveOldMemories(ctx context.Context, idosoID int64, daysOld int, minImportance float64) (int, error) {
	if s.blobAdapter == nil {
		return 0, fmt.Errorf("cold path disabled (S3 not configured)")
	}

	// 1. Localizar memórias candidatas no Postgres
	// Faremos busca por memórias mais antigas que 'daysOld' com importância < 'minImportance'
	memories, err := s.store.GetArchivalCandidates(ctx, idosoID, daysOld, minImportance)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch archival candidates: %w", err)
	}

	count := 0
	for _, m := range memories {
		if m.IsArchived {
			continue
		}

		// 2. Serializar conteúdo completo da memória
		data, err := json.Marshal(m)
		if err != nil {
			log.Printf("⚠️ [COLD-PATH] Falha ao serializar memória %d: %v", m.ID, err)
			continue
		}

		// 3. Upload para S3
		key := fmt.Sprintf("cold-path/idoso-%d/memory-%d.json", idosoID, m.ID)
		err = s.blobAdapter.Upload(ctx, key, data, "application/json")
		if err != nil {
			log.Printf("❌ [COLD-PATH] Falha no upload S3 para memória %d: %v", m.ID, err)
			continue
		}

		// 4. Marcar como arquivado no Postgres (e opcionalmente limpar conteúdo pesado)
		err = s.store.MarkAsArchived(ctx, m.ID)
		if err != nil {
			log.Printf("❌ [COLD-PATH] Falha ao marcar memória %d como arquivada: %v", m.ID, err)
			continue
		}

		count++
	}

	log.Printf("❄️ [COLD-PATH] Ciclo concluído. %d memórias movidas para o arquivo distal.", count)
	return count, nil
}
