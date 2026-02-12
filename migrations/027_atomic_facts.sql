-- =====================================================
-- MIGRATION: ATOMIC FACTS & TEMPORAL GROUNDING
-- =====================================================

-- Adiciona suporte a data real do evento e flag de atomicidade
ALTER TABLE episodic_memories 
ADD COLUMN IF NOT EXISTS event_date TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS is_atomic BOOLEAN DEFAULT FALSE;

-- Index para busca temporal efetiva
CREATE INDEX IF NOT EXISTS idx_memories_event_date ON episodic_memories (idoso_id, event_date DESC);

COMMENT ON COLUMN episodic_memories.event_date IS 'Data em que o fato realmente ocorreu (extraído via LLM)';
COMMENT ON COLUMN episodic_memories.is_atomic IS 'Indica se a memória foi processada e quebrada em fatos atômicos';
