#!/bin/bash
# ============================================================
# EVA-Mind - Auto Redeploy Script
# Roda na VM: git pull + go build + restart services
# Chamado pelo GitHub Actions CI/CD ou manualmente
# ============================================================

set -e

APP_DIR="/home/web2a/EVA-Mind"

echo "=== Redeploy started at $(date) ==="

cd "$APP_DIR"

# Marca diretório como safe para git
git config --global --add safe.directory "$APP_DIR" 2>/dev/null || true

# 1. Pull latest code (reset hard para garantir sync limpo)
echo "[1/4] Syncing code from GitHub..."
git fetch origin main 2>&1
git reset --hard origin/main 2>&1
echo "Commit: $(git log --oneline -1)"

# 2. Build
echo "[2/4] Building..."
export PATH=$PATH:/usr/local/go/bin
CGO_ENABLED=0 go build -o eva-mind . 2>&1
echo "Binary: $(ls -lh eva-mind | awk '{print $5}')"

# 3. Restart EVA-Mind service
echo "[3/4] Restarting eva-mind..."
sudo systemctl restart eva-mind 2>&1

# 4. Health check
echo "[4/4] Health check..."
sleep 4
HTTP=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8091/api/health || echo "000")
if [ "$HTTP" = "200" ]; then
    echo "OK — EVA respondendo (HTTP $HTTP)"
else
    echo "WARNING — Health check retornou HTTP $HTTP"
fi

echo "=== Redeploy complete at $(date) ==="
