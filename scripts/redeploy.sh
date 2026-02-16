#!/bin/bash
# ============================================================
# EVA-Mind - Auto Redeploy Script
# Roda na VM: git pull + go build + restart services
# Chamado pelo GitHub webhook ou manualmente
# ============================================================

APP_DIR="/home/web2a/EVA-Mind"

echo "=== Redeploy started at $(date) ==="

cd "$APP_DIR"

# 1. Pull latest code
echo "[1/5] Pulling latest code..."
git config --global --add safe.directory "$APP_DIR" 2>/dev/null || true
git pull origin main 2>&1

# 2. Build
echo "[2/5] Building..."
export PATH=$PATH:/usr/local/go/bin
go mod tidy 2>&1
CGO_ENABLED=0 go build -ldflags='-s -w' -o eva-mind . 2>&1
echo "Binary: $(ls -lh eva-mind | awk '{print $5}')"

# 3. Restart EVA-Mind service
echo "[3/5] Restarting eva-mind..."
sudo systemctl restart eva-mind 2>&1

# 4. Restart webhook (to pick up new code)
echo "[4/5] Restarting webhook..."
sudo systemctl restart webhook-deploy 2>&1

# 5. Health check
echo "[5/5] Health check..."
sleep 3
curl -s http://localhost:8091/api/health || echo "Health check pending..."
echo ""

echo "=== Redeploy complete at $(date) ==="
