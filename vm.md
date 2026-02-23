# EVA-Mind - VM Deployment Info

## GCP VM (Malaria-Angola)

| Campo | Valor |
|-------|-------|
| **Nome** | `malaria-vm` |
| **Zona** | `africa-south1-a` |
| **Projeto GCP** | `malaria-487614` |
| **IP Interno** | `10.218.0.2` (nic0) |
| **IP Externo** | `34.35.36.178` (nic0) |
| **User** | `web2a` |

## Serviços na VM

| Serviço | Porta | Tipo |
|---------|-------|------|
| **EVA-Mind API** | 8091 | Go binary (systemd) |
| **Malaria API** | 8080 | Go binary (systemd) |
| **Nginx** | 80 | Proxy reverso |
| **NietzscheDB** | 50051 (gRPC) / 8080 (Dashboard) | Docker |

## Databases Externos

| Database | Host | Porta | DB | User |
|----------|------|-------|----|------|
| **PostgreSQL (EVA-Mind)** | `35.232.177.102` | 5432 | `eva-db` | `postgres` |
| **PostgreSQL (Malaria)** | `34.35.142.107` | 5432 | `malaria-db` | `postgres` |

## Acesso SSH

```bash
# Via gcloud
gcloud compute ssh malaria-vm \
  --zone=africa-south1-a \
  --project=malaria-487614

# Via gcloud com comando
gcloud compute ssh malaria-vm \
  --zone=africa-south1-a \
  --project=malaria-487614 \
  --command="<comando>"

# SSH direto (se tiver chave configurada)
ssh web2a@34.35.36.178
```

## Deploy EVA-Mind

```bash
# 1. SSH na VM
gcloud compute ssh malaria-vm --zone=africa-south1-a --project=malaria-487614

# 2. Clonar e rodar deploy
git clone https://github.com/JoseRFJuniorLLMs/EVA-Mind.git
cd EVA-Mind
chmod +x deploy-vm.sh
bash deploy-vm.sh

# 3. Verificar
curl http://localhost:8091/api/health
docker ps
journalctl -u eva-mind -f
```

## Variáveis de Ambiente (EVA-Mind)

```bash
# API
PORT=8091
ENVIRONMENT=production

# Gemini
GOOGLE_API_KEY=AIzaSyBq3AqjhJ4NZv4W9ksN2IAZg-buxKBQi_I
MODEL_ID=gemini-2.5-flash-native-audio-preview-12-2025

# NietzscheDB (Docker local ou remoto)
NIETZSCHE_GRPC_ADDR=localhost:50051
NIETZSCHE_ENCRYPTION_KEY=your-aes-key-here
NIETZSCHE_RBAC_ENABLED=true

# PostgreSQL (Cloud SQL)
DATABASE_URL=postgres://postgres:Debian23%40@35.232.177.102:5432/eva-db?sslmode=disable
```

## Comandos Úteis

```bash
# Logs EVA-Mind
journalctl -u eva-mind -f

# Logs Malaria API
journalctl -u malaria-api -f

# Restart EVA-Mind
sudo systemctl restart eva-mind

# Docker containers
docker ps
docker logs eva-neo4j
docker logs eva-qdrant
docker logs eva-redis

# Neo4j Browser
# http://34.35.36.178:7474

# Qdrant Dashboard
# http://34.35.36.178:6333/dashboard
```
