# Script de Ativação Multimodal EVA (PowerShell)
# Execute após ter Postgres e Qdrant rodando

Write-Host "🚀 Ativando Sistema Multimodal EVA..." -ForegroundColor Green

# 1. Aplicar Migration
Write-Host "`n📦 Step 1/4: Aplicando migration no Postgres..." -ForegroundColor Cyan
psql $env:DATABASE_URL -f "..\EVA-Memory\migrations\028_visual_memories.sql"
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Migration aplicada!" -ForegroundColor Green
} else {
    Write-Host "❌ Erro ao aplicar migration" -ForegroundColor Red
    exit 1
}

# 2. Criar Qdrant Collection
Write-Host "`n📦 Step 2/4: Criando collection no Qdrant..." -ForegroundColor Cyan
$body = @{
    vectors = @{
        size = 64
        distance = "Cosine"
    }
} | ConvertTo-Json

Invoke-RestMethod -Method PUT -Uri "http://localhost:6333/collections/visual_memories_64d" `
    -ContentType "application/json" -Body $body
Write-Host "✅ Collection criada!" -ForegroundColor Green

# 3. Verificar Qdrant
Write-Host "`n📦 Step 3/4: Verificando Qdrant..." -ForegroundColor Cyan
$result = Invoke-RestMethod -Uri "http://localhost:6333/collections/visual_memories_64d"
Write-Host "✅ Qdrant OK! Points: $($result.result.points_count)" -ForegroundColor Green

# 4. Criar arquivo .env.multimodal
Write-Host "`n📦 Step 4/4: Criando arquivo de configuração..." -ForegroundColor Cyan
@"
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
"@ | Out-File -FilePath ".env.multimodal" -Encoding UTF8

Write-Host "✅ Arquivo .env.multimodal criado!" -ForegroundColor Green

Write-Host "`n🎉 ATIVAÇÃO COMPLETA!" -ForegroundColor Green
Write-Host "`nPara testar:" -ForegroundColor Yellow
Write-Host @"
  curl -X POST http://localhost:8091/media/upload?agendamento_id=test-123 `
    -H 'Content-Type: image/jpeg' `
    --data-binary '@test_image.jpg'
"@ -ForegroundColor White
