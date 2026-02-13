#!/bin/bash
set -e

# Exportar Variáveis de Ambiente (Configuração Completa)
export DB_HOST=104.248.219.200
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=Debian23@
export DB_NAME=eva-db
export DATABASE_URL=postgres://postgres:Debian23%40@104.248.219.200:5432/eva-db?sslmode=disable

export NEO4J_URI=bolt://localhost:7687
export NEO4J_USER=neo4j
export NEO4J_PASSWORD=Debian23

export GOOGLE_API_KEY=AIzaSyBlem2g_EFVLTt3Fb1AofF1EOAf05YPo3U

export QDRANT_HOST=localhost
export QDRANT_PORT=6334
export QDRANT_API_KEY=Debian23

export REDIS_HOST=localhost
export REDIS_PORT=6379

export PORT=8091
export ENVIRONMENT=production
export APP_URL=https://eva-ia.org

# Execução
git pull origin main
go mod tidy
go build -o eva-mind main.go

if systemctl list-units --full -all | grep -Fq "eva-mind.service"; then
    systemctl restart eva-mind
else
    pkill eva-mind || true
    nohup ./eva-mind > app.log 2>&1 &
fi
