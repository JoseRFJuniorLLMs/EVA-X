-- Migration: Memory Pipeline Improvements
-- Version: 040
-- Description: Add compressed embeddings, urgency field, and optimized indexes

-- 1. Add compressed embedding column (64D instead of 3072D)
ALTER TABLE episodic_memories 
ADD COLUMN IF NOT EXISTS embedding_compressed FLOAT[] DEFAULT '{}';

-- 2. Add urgency field
ALTER TABLE episodic_memories 
ADD COLUMN IF NOT EXISTS urgency VARCHAR(20) DEFAULT 'MEDIA';

-- 3. Create index on importance for high-value retrieval
CREATE INDEX IF NOT EXISTS idx_episodic_importance 
ON episodic_memories(importance DESC) 
WHERE importance > 0.7;

-- 4. Create index on emotion for emotion-based queries
CREATE INDEX IF NOT EXISTS idx_episodic_emotion 
ON episodic_memories(emotion) 
WHERE emotion != 'neutral';

-- 5. Create index on urgency for critical memory retrieval
CREATE INDEX IF NOT EXISTS idx_episodic_urgency 
ON episodic_memories(urgency) 
WHERE urgency IN ('ALTA', 'CRITICA');

-- 6. Create index on is_atomic for atomic fact queries
CREATE INDEX IF NOT EXISTS idx_episodic_atomic 
ON episodic_memories(is_atomic) 
WHERE is_atomic = true;

-- 7. Add comment for documentation
COMMENT ON COLUMN episodic_memories.embedding_compressed IS 'Krylov-compressed embedding (64D) for efficient storage';
COMMENT ON COLUMN episodic_memories.urgency IS 'Urgency level from AudioAnalysis: BAIXA, MEDIA, ALTA, CRITICA';

COMMIT;
