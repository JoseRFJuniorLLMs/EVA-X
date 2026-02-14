#!/bin/bash
# Deploy EVA-Mind to GCP VM (aurora-sadtalker)
# IP: 136.113.25.218

set -e

echo "🚀 Deploying EVA-Mind to GCP VM..."

# SSH to VM and deploy
gcloud compute ssh aurora-sadtalker \
  --zone=us-central1-a \
  --project=aurorav2-484411 \
  --tunnel-through-iap \
  --command="
    cd ~/EVA-Mind || exit 1
    echo '📥 Pulling latest code...'
    git pull origin main
    
    echo '🔧 Building application...'
    go mod tidy
    go build -o eva-mind .
    
    echo '🔄 Restarting service...'
    if systemctl list-units --full -all | grep -Fq 'eva-mind.service'; then
        sudo systemctl restart eva-mind
        echo '✅ Service restarted'
    else
        pkill eva-mind || true
        nohup ./eva-mind > app.log 2>&1 &
        echo '✅ Service started in background'
    fi
    
    echo '✅ Deployment complete!'
    sleep 2
    
    echo '📊 Service status:'
    systemctl status eva-mind --no-pager || ps aux | grep eva-mind | grep -v grep
  "

echo ""
echo "✅ EVA-Mind deployed successfully to GCP VM!"
echo "🌐 API: http://136.113.25.218:8000"
