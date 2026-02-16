#!/bin/bash
# Log to both file and serial port (syslog)
exec > >(tee -a /var/log/eva-mind-deploy.log) 2>&1
echo "=== EVA-Mind Deploy Started at $(date) ==="

# Expand disk filesystem
resize2fs /dev/sda1 2>/dev/null || true

export HOME=/home/web2a
export USER=web2a
REPO_URL="https://github.com/JoseRFJuniorLLMs/EVA-Mind.git"
APP_DIR="/home/web2a/EVA-Mind"
GO_VERSION="1.24.0"
DB_URL="postgres://postgres:Debian23%40@34.35.142.107:5432/eva-mind?sslmode=disable"

# 1. Docker
if ! command -v docker &> /dev/null; then
    echo "[1/7] Installing Docker..."
    apt-get update -qq
    apt-get install -y -qq ca-certificates curl gnupg
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
    CODENAME=$(. /etc/os-release && echo "$VERSION_CODENAME")
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $CODENAME stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
    apt-get update -qq
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
    usermod -aG docker web2a
else
    echo "[1/7] Docker OK"
fi

# 2. Go
if ! /usr/local/go/bin/go version 2>/dev/null | grep -q "go1.24"; then
    echo "[2/7] Installing Go ${GO_VERSION}..."
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
else
    echo "[2/7] Go OK"
fi
export PATH=$PATH:/usr/local/go/bin:/home/web2a/go/bin

# 3. Clone/update repo
git config --global --add safe.directory "$APP_DIR" 2>/dev/null || true
if [ -d "$APP_DIR" ]; then
    echo "[3/7] Updating EVA-Mind..."
    cd "$APP_DIR"
    git pull origin main || true
else
    echo "[3/7] Cloning EVA-Mind..."
    git clone "$REPO_URL" "$APP_DIR"
fi
chown -R web2a:web2a "$APP_DIR"
cd "$APP_DIR"

# 4. Docker infra
echo "[4/7] Starting Docker containers..."
docker compose -f docker-compose.infra.yml up -d

echo "Waiting for services..."
for i in $(seq 1 30); do
    NEO4J_OK=$(docker inspect --format='{{.State.Health.Status}}' eva-neo4j 2>/dev/null || echo "starting")
    QDRANT_OK=$(docker inspect --format='{{.State.Health.Status}}' eva-qdrant 2>/dev/null || echo "starting")
    REDIS_OK=$(docker inspect --format='{{.State.Health.Status}}' eva-redis 2>/dev/null || echo "starting")
    if [ "$NEO4J_OK" = "healthy" ] && [ "$QDRANT_OK" = "healthy" ] && [ "$REDIS_OK" = "healthy" ]; then
        echo "All services healthy!"
        break
    fi
    echo "  Neo4j=$NEO4J_OK Qdrant=$QDRANT_OK Redis=$REDIS_OK ($i/30)"
    sleep 5
done

# 5. Create .env
echo "[5/7] Creating .env..."
cat > "$APP_DIR/.env" << 'ENVEOF'
APP_NAME=EVA Mind
PORT=8091
ENVIRONMENT=production
METRICS_PORT=9090
DATABASE_URL=postgres://postgres:Debian23%40@34.35.142.107:5432/eva-mind?sslmode=disable
DB_HOST=34.35.142.107
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=Debian23@
DB_NAME=eva-mind
DB_SSLMODE=disable
GOOGLE_API_KEY=AIzaSyBq3AqjhJ4NZv4W9ksN2IAZg-buxKBQi_I
MODEL_ID=gemini-2.5-flash-native-audio-preview-12-2025
GEMINI_ANALYSIS_MODEL=gemini-3-flash
GEMINI_MODEL_FAST=gemini-3-flash
GEMINI_MODEL_SMART=gemini-3-pro
VISION_MODEL_ID=gemini-2.0-flash-exp
JWT_SECRET=EVA-Mind-2026-K8s-Pr0d-S3cr3t-X9f2mQ7wR4tY6uI0pL
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_USERNAME=neo4j
NEO4J_PASSWORD=Debian23
QDRANT_HOST=localhost
QDRANT_PORT=6334
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
SERVICE_DOMAIN=34.35.36.178:8091
TWILIO_ACCOUNT_SID=AC4ec3781eec6990a67014c74a9580a705
TWILIO_AUTH_TOKEN=0e1edb3a1242d17901c62d5119aa51f0
TWILIO_PHONE_NUMBER=+351966805210
ENABLE_SMS_FALLBACK=false
ENABLE_CALL_FALLBACK=false
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=web2ajax@gmail.com
SMTP_PASSWORD=@Debian23@
SMTP_FROM_NAME=EVA - Assistente Virtual
SMTP_FROM_EMAIL=web2ajax@gmail.com
ENABLE_EMAIL_FALLBACK=true
FIREBASE_CREDENTIALS_PATH=serviceAccountKey.json
GOOGLE_CLIENT_ID=1017997949026-6icl937adoggb08v9fnk14spj2is8q22.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=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
GOOGLE_REDIRECT_URL=https://api.eva-ia.org/api/oauth/google/callback
SCHEDULER_INTERVAL=1
MAX_RETRIES=3
ALERT_RETRY_INTERVAL=5
ALERT_ESCALATION_TIME=5
CRITICAL_ALERT_TIMEOUT=5
APP_URL=http://34.35.36.178:8091
API_BASE_URL=http://34.35.36.178:8091/api
WS_URL=ws://34.35.36.178:8091/ws/pcm
WS_PCM_URL=ws://34.35.36.178:8091/ws/pcm
ENVEOF
chown web2a:web2a "$APP_DIR/.env"

# 5b. Migrations
echo "[5b] Running migrations..."
apt-get install -y -qq postgresql-client 2>/dev/null || true
MIGRATION_DIR="$APP_DIR/migrations"
export PGPASSWORD='Debian23@'
PG_CONN="-h 34.35.142.107 -p 5432 -U postgres -d eva-mind"
for migration in \
    001_initial.sql \
    001_create_device_tokens_table.sql \
    002_add_session_handle.sql \
    002_clinical_and_vision_features.sql \
    003_system_upgrades.sql \
    003_cognitive_load_and_ethical_boundaries.sql \
    004_clinical_decision_explainer.sql \
    005_predictive_trajectory.sql \
    007_clinical_research_engine.sql \
    008_multi_persona_system.sql \
    008_persona_seed_data.sql \
    009_exit_protocol.sql \
    010_integration_layer.sql \
    011_escalation_logs.sql \
    012_superhuman_memory_system.sql \
    013_deep_memory_extensions.sql \
    014_superhuman_consciousness.sql \
    015_critical_memory_systems.sql \
    016_dynamic_tools_system.sql \
    016_dynamic_tools_seed.sql \
    017_eva_self_knowledge.sql \
    017_eva_self_knowledge_seed.sql \
    017_entertainment_tools_seed.sql \
    018_estilo_conversa.sql \
    018_lgpd_audit_trail.sql \
    019_perfil_criador.sql \
    020_lacan_tables.sql \
    021_enneagram_tables.sql \
    022_system_prompts.sql \
    023_historico_medicamentos.sql \
    024_performance_indexes.sql \
    025_idioma_idoso.sql \
    026_legacy_mode.sql \
    027_atomic_facts.sql \
    030_architect_override_and_personal_info.sql \
    035_add_multitenancy.sql \
    036_add_atomic_facts_dual_timestamp.sql \
    037_add_clinical_psychology.sql \
    038_add_crisis_synthesis_family.sql \
    040_memory_improvements.sql; do
    if [ -f "$MIGRATION_DIR/$migration" ]; then
        echo "  Running: $migration"
        psql $PG_CONN -f "$MIGRATION_DIR/$migration" 2>&1 | tail -1 || echo "  WARNING: $migration may already be applied"
    fi
done

# 6. Build
echo "[6/7] Building EVA-Mind..."
cd "$APP_DIR"
export PATH=$PATH:/usr/local/go/bin:/home/web2a/go/bin
export GOPATH=/home/web2a/go
export GOCACHE=/home/web2a/.cache/go-build
chown -R web2a:web2a "$APP_DIR"
su - web2a -c "export PATH=\$PATH:/usr/local/go/bin && cd $APP_DIR && go mod tidy 2>&1 && CGO_ENABLED=0 go build -ldflags='-s -w' -o eva-mind . 2>&1" || {
    echo "ERROR: Go build failed! Trying as root..."
    cd "$APP_DIR"
    /usr/local/go/bin/go mod tidy 2>&1
    CGO_ENABLED=0 /usr/local/go/bin/go build -ldflags="-s -w" -o eva-mind . 2>&1
    chown web2a:web2a "$APP_DIR/eva-mind"
}
ls -lh "$APP_DIR/eva-mind" 2>/dev/null && echo "Build OK" || echo "ERROR: Binary not found!"

# 7. Systemd service
echo "[7/7] Creating systemd service..."
cat > /etc/systemd/system/eva-mind.service << 'SVCEOF'
[Unit]
Description=EVA-Mind API Server
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
User=web2a
WorkingDirectory=/home/web2a/EVA-Mind
EnvironmentFile=/home/web2a/EVA-Mind/.env
ExecStart=/home/web2a/EVA-Mind/eva-mind
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SVCEOF

systemctl daemon-reload
systemctl enable eva-mind
systemctl restart eva-mind

# 7b. Allow web2a to restart services without password
echo "web2a ALL=(ALL) NOPASSWD: /bin/systemctl restart eva-mind, /bin/systemctl restart webhook-deploy, /bin/systemctl status eva-mind, /bin/journalctl *" > /etc/sudoers.d/eva-mind
chmod 440 /etc/sudoers.d/eva-mind

# 8. Webhook deploy server (auto-deploy on git push)
echo "[8/8] Setting up webhook deploy server..."
cat > /etc/systemd/system/webhook-deploy.service << 'WHEOF'
[Unit]
Description=EVA-Mind Webhook Deploy Server
After=network.target

[Service]
Type=simple
User=web2a
WorkingDirectory=/home/web2a/EVA-Mind
ExecStart=/usr/bin/python3 /home/web2a/EVA-Mind/scripts/webhook-server.py
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
WHEOF

systemctl daemon-reload
systemctl enable webhook-deploy
systemctl restart webhook-deploy

sleep 3
echo "=== DEPLOY COMPLETE ==="
echo "Docker:"
docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "(eva|NAMES)"
echo ""
echo "EVA-Mind:"
systemctl is-active eva-mind
echo ""
echo "Webhook:"
systemctl is-active webhook-deploy
echo ""
echo "Health:"
curl -s http://localhost:8091/api/health || echo "Waiting..."
echo ""
echo "=== EVA-Mind Deploy Finished at $(date) ==="
