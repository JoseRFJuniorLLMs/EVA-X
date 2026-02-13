#!/bin/bash
# Deploy Simples para VPS (Linux)
set -e

echo "🚀 Iniciando deploy..."

# 1. Atualizar código
git pull origin main

# 2. Baixar dependências
go mod tidy

# 3. Compilar
go build -o eva-mind main.go

# 4. Reiniciar serviço
# Tenta reiniciar via systemctl, se falhar avisa
if systemctl list-units --full -all | grep -Fq "eva-mind.service"; then
    systemctl restart eva-mind
    echo "✅ Serviço reiniciado com sucesso!"
else
    echo "⚠️  Serviço 'eva-mind' não encontrado no systemd."
    echo "   Para rodar manual: nohup ./eva-mind > app.log 2>&1 &"
fi

echo "✅ Deploy concluído."
