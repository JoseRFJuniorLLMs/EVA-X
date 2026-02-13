-- Remover coluna de embedding do Postgres (agora 100% Qdrant)
ALTER TABLE episodic_memories DROP COLUMN IF EXISTS embedding;

-- Remover função de busca antiga (agora via Qdrant)
DROP FUNCTION IF EXISTS search_similar_memories;
