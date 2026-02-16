#!/bin/bash
# ============================================================
# EVA-Mind - Auto Redeploy Script
# Roda na VM: git pull + go build + restart service
# Chamado pelo GitHub webhook ou manualmente
# ============================================================
set -e

APP_DIR="/home/web2a/EVA-Mind"
LOG_FILE="/var/log/eva-mind-redeploy.log"

echo "=== Redeploy started at $(date) ===" | tee -a "$LOG_FILE"

cd "$APP_DIR"

# 1. Pull latest code
echo "[1/4] Pulling latest code..." | tee -a "$LOG_FILE"
git config --global --add safe.directory "$APP_DIR" 2>/dev/null || true
git pull origin main 2>&1 | tee -a "$LOG_FILE"

# 2. Build
echo "[2/4] Building..." | tee -a "$LOG_FILE"
export PATH=$PATH:/usr/local/go/bin
go mod tidy 2>&1 | tee -a "$LOG_FILE"
CGO_ENABLED=0 go build -ldflags='-s -w' -o eva-mind . 2>&1 | tee -a "$LOG_FILE"
echo "Binary: $(ls -lh eva-mind | awk '{print $5}')" | tee -a "$LOG_FILE"

# 3. Restart service
echo "[3/4] Restarting service..." | tee -a "$LOG_FILE"
sudo systemctl restart eva-mind 2>&1 | tee -a "$LOG_FILE"

# 4. Health check
echo "[4/4] Health check..." | tee -a "$LOG_FILE"
sleep 3
curl -s http://localhost:8091/api/health | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

echo "=== Redeploy complete at $(date) ===" | tee -a "$LOG_FILE"
