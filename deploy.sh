#!/bin/bash
# ============================================================
# EVA-Mind - Deploy to Google Cloud Run
# ============================================================
# Usage: ./deploy.sh [project-id] [region]
# Default: project from gcloud config, region=southamerica-east1
# ============================================================

set -euo pipefail

# Configuration
PROJECT_ID="${1:-$(gcloud config get-value project 2>/dev/null)}"
REGION="${2:-southamerica-east1}"
SERVICE_NAME="eva-mind"
IMAGE="gcr.io/${PROJECT_ID}/${SERVICE_NAME}"

# Version info
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo "============================================"
echo "  EVA-Mind Deploy to Cloud Run"
echo "============================================"
echo "  Project:  ${PROJECT_ID}"
echo "  Region:   ${REGION}"
echo "  Service:  ${SERVICE_NAME}"
echo "  Version:  ${VERSION}"
echo "  Commit:   ${GIT_COMMIT}"
echo "============================================"

# Step 1: Build
echo ""
echo "[1/3] Building Docker image..."
docker build \
    --build-arg VERSION="${VERSION}" \
    --build-arg GIT_COMMIT="${GIT_COMMIT}" \
    --build-arg BUILD_TIME="${BUILD_TIME}" \
    -t "${IMAGE}:${GIT_COMMIT}" \
    -t "${IMAGE}:latest" \
    .

# Step 2: Push
echo ""
echo "[2/3] Pushing to Container Registry..."
docker push "${IMAGE}:${GIT_COMMIT}"
docker push "${IMAGE}:latest"

# Step 3: Deploy
echo ""
echo "[3/3] Deploying to Cloud Run..."
gcloud run deploy "${SERVICE_NAME}" \
    --image="${IMAGE}:${GIT_COMMIT}" \
    --region="${REGION}" \
    --platform=managed \
    --port=8091 \
    --memory=1Gi \
    --cpu=2 \
    --min-instances=1 \
    --max-instances=10 \
    --timeout=3600 \
    --concurrency=80 \
    --allow-unauthenticated \
    --session-affinity \
    --set-env-vars="ENVIRONMENT=production" \
    --set-env-vars="PORT=8091" \
    --quiet

# Get URL
SERVICE_URL=$(gcloud run services describe "${SERVICE_NAME}" \
    --region="${REGION}" \
    --format='value(status.url)' 2>/dev/null)

echo ""
echo "============================================"
echo "  Deploy Complete!"
echo "  URL: ${SERVICE_URL}"
echo "============================================"
echo ""
echo "Next steps:"
echo "  1. Set secrets: gcloud run services update ${SERVICE_NAME} --region=${REGION} \\"
echo "       --set-env-vars='DB_HOST=104.248.219.200,DB_PASSWORD=...,GOOGLE_API_KEY=...'"
echo "  2. Map domain: gcloud run domain-mappings create --service=${SERVICE_NAME} \\"
echo "       --domain=eva-ia.org --region=${REGION}"
echo "  3. Test: curl ${SERVICE_URL}/api/health"
