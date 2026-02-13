#!/bin/bash
# Deploy de Recuperação (VPS)
# Este script tenta restaurar as configurações perdidas e reiniciar o serviço.

set -e

echo "� Iniciando Deploy de Recuperação..."

# 1. Restaurar arquivo .env (Candidatos encontrados no histórico)
# IP 104.248... encontrado em api_server.py
# Senha Debian23 encontrada em vm.env
echo "📝 Recriando .env..."
cat <<EOF > .env
# Database (Postgres)
DB_HOST=104.248.219.200
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=Debian23
DB_NAME=eva_mind

# Neo4j
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=Debian23

# API Keys (Google Gemini)
# Opção 1 (vm.env):
GOOGLE_API_KEY=AIzaSyAJq7G4wg_7GSlz1CmgKxqCtLlkzQ3YmTQ
# Opção 2 (backup): 
# GOOGLE_API_KEY=AIzaSyCL5RwY9F5QjzOiLNiOXAN4JjelhrB3S1U

# Qdrant
QDRANT_HOST=localhost
QDRANT_PORT=6333
EOF

# 2. Atualizar e Compilar
echo "📥 Git Pull..."
git pull origin main

echo "📦 Go Mod Tidy..."
# go mod tidy  <-- Comentado para agilizar se a net estiver lenta, descomente se precisar

echo "🔨 Compilando..."
go build -o eva-mind main.go

# 3. Reiniciar
echo "🔄 Reiniciando serviço..."
if systemctl list-units --full -all | grep -Fq "eva-mind.service"; then
    systemctl restart eva-mind
    echo "✅ Serviço reiniciado!"
else
    echo "⚠️ Systemd unit não encontrada. Tentando rodar manual em background..."
    pkill eva-mind || true
    nohup ./eva-mind > app.log 2>&1 &
    echo "✅ Rodando em background (nohup)"
fi

echo "📜 Verifique os logs: tail -f app.log ou journalctl -u eva-mind -f"
