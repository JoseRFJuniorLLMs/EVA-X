#!/bin/bash
# Script de Ativação Multimodal EVA
# Execute após ter Postgres e Qdrant rodando

set -e

echo "🚀 Ativando Sistema Multimodal EVA..."

# 1. Aplicar Migration
echo "📦 Step 1/4: Aplicando migration no Postgres..."
psql "$DATABASE_URL" -f ../EVA-Memory/migrations/028_visual_memories.sql
echo "✅ Migration aplicada!"

# 2. Criar Qdrant Collection
echo "📦 Step 2/4: Criando collection no Qdrant..."
curl -X PUT http://localhost:6333/collections/visual_memories_64d \
  -H 'Content-Type: application/json' \
  -d '{
    "vectors": {
      "size": 64,
      "distance": "Cosine"
    }
  }'
echo "✅ Collection criada!"

# 3. Verificar Qdrant
echo "📦 Step 3/4: Verificando Qdrant..."
curl http://localhost:6333/collections/visual_memories_64d
echo "✅ Qdrant OK!"

# 4. Configurar Env Vars
echo "📦 Step 4/4: Configurando variáveis de ambiente..."
cat > .env.multimodal <<'EOF'
# Multimodal Configuration
EVA_MULTIMODAL_ENABLED=true
EVA_MULTIMODAL_FEATURES_IMAGE_UPLOAD=true
EVA_MULTIMODAL_FEATURES_VIDEO_UPLOAD=false
EVA_MULTIMODAL_FEATURES_VIDEO_STREAMING=false
EVA_MULTIMODAL_FEATURES_VISUAL_MEMORY=true
EVA_MULTIMODAL_FEATURES_HYBRID_RETRIEVAL=true

# Limits
EVA_MULTIMODAL_LIMITS_MAX_IMAGE_SIZE_MB=7
EVA_MULTIMODAL_LIMITS_MAX_VIDEO_SIZE_MB=30
EVA_MULTIMODAL_LIMITS_VIDEO_FRAME_RATE_FPS=1

# Quality
EVA_MULTIMODAL_QUALITY_IMAGE_COMPRESSION=85

# Memory
EVA_MULTIMODAL_MEMORY_FLUSH_INTERVAL_SEC=30
EVA_MULTIMODAL_MEMORY_BATCH_SIZE=10

# Retrieval
EVA_MULTIMODAL_RETRIEVAL_TEXT_WEIGHT=0.7
EVA_MULTIMODAL_RETRIEVAL_VISUAL_WEIGHT=0.3
EOF

echo "✅ Arquivo .env.multimodal criado!"
echo ""
echo "🎉 ATIVAÇÃO COMPLETA!"
echo ""
echo "Para usar, adicione ao seu .env:"
echo "  source .env.multimodal"
echo ""
echo "Para testar:"
echo "  curl -X POST http://localhost:8091/media/upload?agendamento_id=test-123 \\"
echo "    -H 'Content-Type: image/jpeg' \\"
echo "    --data-binary @test_image.jpg"
