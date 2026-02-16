#!/bin/bash
set -e

# ============================================================
# EVA-Mind - Deploy Completo na VM (Malaria-Angola)
# Neo4j + Qdrant + Redis + Go build + systemd
# ============================================================

REPO_URL="https://github.com/JoseRFJuniorLLMs/EVA-Mind.git"
APP_DIR="$HOME/EVA-Mind"
GO_VERSION="1.24.0"
DB_URL="postgres://postgres:Debian23%40@35.232.177.102:5432/eva-db?sslmode=disable"

echo "=========================================="
echo "  EVA-Mind - Deploy na VM"
echo "=========================================="

# ------------------------------------------
# 1. Instalar Docker (se não existir)
# ------------------------------------------
if ! command -v docker &> /dev/null; then
    echo "[1/7] Instalando Docker..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq ca-certificates curl gnupg
    sudo install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    sudo chmod a+r /etc/apt/keyrings/docker.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update -qq
    sudo apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
    sudo usermod -aG docker $USER
    echo "Docker instalado. Se acabou de adicionar ao grupo docker, faça logout/login e rode novamente."
else
    echo "[1/7] Docker OK"
fi

# ------------------------------------------
# 2. Instalar Go (se não existir ou versão antiga)
# ------------------------------------------
if ! command -v go &> /dev/null || [[ "$(go version 2>/dev/null)" != *"go1.24"* ]]; then
    echo "[2/7] Instalando Go ${GO_VERSION}..."
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz

    # Adicionar ao PATH se não estiver
    if ! grep -q '/usr/local/go/bin' ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
    fi
    export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
    echo "Go $(go version) instalado"
else
    echo "[2/7] Go OK - $(go version)"
fi

# ------------------------------------------
# 3. Clonar ou atualizar EVA-Mind do GitHub
# ------------------------------------------
if [ -d "$APP_DIR" ]; then
    echo "[3/7] Atualizando EVA-Mind (git pull)..."
    cd "$APP_DIR"
    git pull origin main
else
    echo "[3/7] Clonando EVA-Mind do GitHub..."
    git clone "$REPO_URL" "$APP_DIR"
    cd "$APP_DIR"
fi

# ------------------------------------------
# 4. Subir Neo4j, Qdrant, Redis via Docker
# ------------------------------------------
echo "[4/7] Subindo infraestrutura (Neo4j, Qdrant, Redis)..."
cd "$APP_DIR"
sudo docker compose -f docker-compose.infra.yml up -d

# Aguardar serviços ficarem prontos
echo "Aguardando serviços..."
for i in {1..30}; do
    NEO4J_OK=$(sudo docker inspect --format='{{.State.Health.Status}}' eva-neo4j 2>/dev/null || echo "starting")
    QDRANT_OK=$(sudo docker inspect --format='{{.State.Health.Status}}' eva-qdrant 2>/dev/null || echo "starting")
    REDIS_OK=$(sudo docker inspect --format='{{.State.Health.Status}}' eva-redis 2>/dev/null || echo "starting")

    if [ "$NEO4J_OK" = "healthy" ] && [ "$QDRANT_OK" = "healthy" ] && [ "$REDIS_OK" = "healthy" ]; then
        echo "Todos os serviços estão prontos!"
        break
    fi
    echo "  Neo4j=$NEO4J_OK Qdrant=$QDRANT_OK Redis=$REDIS_OK (tentativa $i/30)"
    sleep 5
done

# ------------------------------------------
# 5. Criar .env
# ------------------------------------------
echo "[5/7] Criando .env..."
cat > "$APP_DIR/.env" << 'ENVEOF'
APP_NAME=EVA Mind
PORT=8091
ENVIRONMENT=production
METRICS_PORT=9090

# PostgreSQL (Cloud SQL)
DATABASE_URL=postgres://postgres:Debian23%40@35.232.177.102:5432/eva-db?sslmode=disable
DB_HOST=35.232.177.102
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=Debian23@
DB_NAME=eva-db
DB_SSLMODE=disable

# Google/Gemini
GOOGLE_API_KEY=AIzaSyBq3AqjhJ4NZv4W9ksN2IAZg-buxKBQi_I
MODEL_ID=gemini-2.5-flash-native-audio-preview-12-2025
GEMINI_ANALYSIS_MODEL=gemini-3-flash
GEMINI_MODEL_FAST=gemini-3-flash
GEMINI_MODEL_SMART=gemini-3-pro
VISION_MODEL_ID=gemini-2.0-flash-exp

# JWT
JWT_SECRET=EVA-Mind-2026-K8s-Pr0d-S3cr3t-X9f2mQ7wR4tY6uI0pL

# Neo4j (Docker local)
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_USERNAME=neo4j
NEO4J_PASSWORD=Debian23

# Qdrant (Docker local)
QDRANT_HOST=localhost
QDRANT_PORT=6334

# Redis (Docker local)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# Twilio
SERVICE_DOMAIN=eva-ia.org:8080
TWILIO_ACCOUNT_SID=AC4ec3781eec6990a67014c74a9580a705
TWILIO_AUTH_TOKEN=0e1edb3a1242d17901c62d5119aa51f0
TWILIO_PHONE_NUMBER=+351966805210
ENABLE_SMS_FALLBACK=false
ENABLE_CALL_FALLBACK=false

# SMTP
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=web2ajax@gmail.com
SMTP_PASSWORD=@Debian23@
SMTP_FROM_NAME=EVA - Assistente Virtual
SMTP_FROM_EMAIL=web2ajax@gmail.com
ENABLE_EMAIL_FALLBACK=true

# Firebase
FIREBASE_CREDENTIALS_PATH=serviceAccountKey.json

# Google OAuth
GOOGLE_CLIENT_ID=1017997949026-6icl937adoggb08v9fnk14spj2is8q22.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=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
GOOGLE_REDIRECT_URL=https://api.eva-ia.org/api/oauth/google/callback

# Scheduler
SCHEDULER_INTERVAL=1
MAX_RETRIES=3

# Alert System
ALERT_RETRY_INTERVAL=5
ALERT_ESCALATION_TIME=5
CRITICAL_ALERT_TIMEOUT=5

# App URLs
APP_URL=https://eva-ia.org
API_BASE_URL=https://eva-ia.org:8000/api/v1
WS_URL=wss://eva-ia.org:8090/ws/pcm
WS_PCM_URL=wss://eva-ia.org:8090/ws/pcm
ENVEOF

echo ".env criado com GOOGLE_API_KEY=AIzaSyBq3AqjhJ4NZv4W9ksN2IAZg-buxKBQi_I"

# ------------------------------------------
# 5b. Rodar migrations SQL no PostgreSQL
# ------------------------------------------
echo "[5b/8] Rodando migrations SQL..."
if command -v psql &> /dev/null; then
    PSQL_CMD="psql"
else
    sudo apt-get install -y -qq postgresql-client
    PSQL_CMD="psql"
fi

MIGRATION_DIR="$APP_DIR/migrations"
MIGRATION_ORDER=(
    "001_initial.sql"
    "001_create_device_tokens_table.sql"
    "002_add_session_handle.sql"
    "002_clinical_and_vision_features.sql"
    "003_system_upgrades.sql"
    "003_cognitive_load_and_ethical_boundaries.sql"
    "004_clinical_decision_explainer.sql"
    "005_predictive_trajectory.sql"
    "007_clinical_research_engine.sql"
    "008_multi_persona_system.sql"
    "008_persona_seed_data.sql"
    "009_exit_protocol.sql"
    "010_integration_layer.sql"
    "011_escalation_logs.sql"
    "012_superhuman_memory_system.sql"
    "013_deep_memory_extensions.sql"
    "014_superhuman_consciousness.sql"
    "015_critical_memory_systems.sql"
    "016_dynamic_tools_system.sql"
    "016_dynamic_tools_seed.sql"
    "017_eva_self_knowledge.sql"
    "017_eva_self_knowledge_seed.sql"
    "017_entertainment_tools_seed.sql"
    "018_estilo_conversa.sql"
    "018_lgpd_audit_trail.sql"
    "019_perfil_criador.sql"
    "020_lacan_tables.sql"
    "021_enneagram_tables.sql"
    "022_system_prompts.sql"
    "023_historico_medicamentos.sql"
    "024_performance_indexes.sql"
    "025_idioma_idoso.sql"
    "026_legacy_mode.sql"
    "027_atomic_facts.sql"
    "030_architect_override_and_personal_info.sql"
    "035_add_multitenancy.sql"
    "036_add_atomic_facts_dual_timestamp.sql"
    "037_add_clinical_psychology.sql"
    "038_add_crisis_synthesis_family.sql"
    "040_memory_improvements.sql"
)

for migration in "${MIGRATION_ORDER[@]}"; do
    if [ -f "$MIGRATION_DIR/$migration" ]; then
        echo "  Rodando: $migration"
        $PSQL_CMD "$DB_URL" -f "$MIGRATION_DIR/$migration" 2>&1 | tail -1 || echo "  AVISO: $migration pode já ter sido aplicada"
    fi
done
echo "Migrations concluídas!"

# ------------------------------------------
# 6. Build do Go
# ------------------------------------------
echo "[6/7] Compilando EVA-Mind..."
cd "$APP_DIR"
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
go mod tidy
CGO_ENABLED=0 go build -ldflags="-s -w" -o eva-mind .
echo "Build OK - $(ls -lh eva-mind | awk '{print $5}')"

# ------------------------------------------
# 7. Criar systemd service e iniciar
# ------------------------------------------
echo "[7/7] Configurando serviço systemd..."
sudo tee /etc/systemd/system/eva-mind.service > /dev/null << EOF
[Unit]
Description=EVA-Mind API Server
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
User=$USER
WorkingDirectory=$APP_DIR
EnvironmentFile=$APP_DIR/.env
ExecStart=$APP_DIR/eva-mind
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable eva-mind
sudo systemctl restart eva-mind

# Aguardar startup
sleep 3

# ------------------------------------------
# Verificação final
# ------------------------------------------
echo ""
echo "=========================================="
echo "  Verificação"
echo "=========================================="

echo "Docker containers:"
sudo docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep eva

echo ""
echo "EVA-Mind service:"
sudo systemctl status eva-mind --no-pager -l | head -15

echo ""
echo "Health check:"
curl -s http://localhost:8091/api/health || echo "ERRO: Health check falhou"

echo ""
echo "=========================================="
echo "  Deploy completo!"
echo "  API: http://localhost:8091"
echo "  Neo4j Browser: http://localhost:7474"
echo "  Qdrant Dashboard: http://localhost:6333/dashboard"
echo "=========================================="
