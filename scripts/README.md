# 🚀 EVA Core Memory - Scripts de Carga

Scripts para inicializar a memória e conhecimento de EVA.

---

## 📁 Arquivos

### 1. `seed_eva_memory.sh` (Bash)
Script shell completo que carrega conhecimento fundamental via curl.

**Requisitos:**
- `bash`
- `curl`
- `jq` (opcional, para output formatado)

**Uso:**
```bash
# Permissão de execução
chmod +x seed_eva_memory.sh

# Executar (servidor rodando em localhost:8080)
./seed_eva_memory.sh

# Ou com URL customizada
EVA_API_URL=http://192.168.1.100:8080 ./seed_eva_memory.sh
```

### 2. `load_knowledge.py` (Python)
Script Python que carrega conhecimento de arquivo JSON.

**Requisitos:**
- Python 3.7+
- `requests` library

**Instalação:**
```bash
pip install requests
```

**Uso:**
```bash
# Executar com arquivo padrão
python3 load_knowledge.py

# Ou especificar arquivo JSON
python3 load_knowledge.py ../data/eva_initial_knowledge.json

# Com URL customizada
python3 load_knowledge.py ../data/eva_initial_knowledge.json http://192.168.1.100:8080
```

### 3. `../data/eva_initial_knowledge.json`
Arquivo JSON com conhecimento inicial estruturado (34 itens).

**Estrutura:**
```json
{
  "metadata": {
    "version": "1.0",
    "created_at": "2025-02-16",
    "description": "...",
    "total_items": 34
  },
  "lessons": [...],
  "patterns": [...],
  "emotional_rules": [...],
  "meta_insights": [...],
  "safety_protocols": [...]
}
```

---

## 🎯 Quick Start

### Opção 1: Bash (Rápido)

```bash
cd scripts
chmod +x seed_eva_memory.sh
./seed_eva_memory.sh
```

### Opção 2: Python (Flexível)

```bash
cd scripts
pip install requests
python3 load_knowledge.py
```

---

## 📊 O Que É Carregado

### Lições Fundamentais (8 itens)
- Empatia como base terapêutica
- Gestão de crises
- Proteção de privacidade
- Escuta ativa
- Etc.

### Padrões Comportamentais (7 itens)
- Ansiedade noturna
- Isolamento social
- Mudanças de humor
- Etc.

### Regras Emocionais (8 itens)
- Validação de emoções
- Perguntas abertas
- Presença empática
- Etc.

### Meta-Insights (6 itens)
- Necessidade de ser ouvido
- Segurança psicológica
- Conexão genuína
- Etc.

### Protocolos de Segurança (5 itens)
- Ideação suicida
- Sintomas de psicose
- Abuso ativo
- Automutilação
- Dependência química

**Total: 34 memórias iniciais**

---

## 🔍 Verificação Pós-Carga

### Ver Memórias Carregadas

```bash
# Estatísticas
curl http://localhost:8080/self/memories/stats

# Listar memórias
curl http://localhost:8080/self/memories

# Personalidade
curl http://localhost:8080/self/personality

# Identidade (priming)
curl http://localhost:8080/self/identity
```

### Buscar Memórias

```bash
curl -X POST http://localhost:8080/self/memories/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "ansiedade",
    "top_k": 5
  }'
```

---

## 🛠️ Customização

### Adicionar Nova Memória

Edite `../data/eva_initial_knowledge.json` e adicione:

```json
{
  "content": "Nova lição aqui",
  "category": "lesson",
  "importance": 0.85
}
```

### Categorias Disponíveis

| Categoria | Uso | Importance Recomendado |
|-----------|-----|------------------------|
| `lesson` | Lição específica | 0.7-0.9 |
| `pattern` | Padrão observado | 0.8-0.95 |
| `meta_insight` | Insight de alto nível | 0.9-1.0 |
| `emotional_rule` | Regra emocional | 0.85-1.0 |
| `self_critique` | Auto-avaliação | 0.6-0.8 |

### Criar Seu Próprio JSON

Copie o template e customize:

```bash
cp ../data/eva_initial_knowledge.json ../data/my_custom_knowledge.json
# Edite my_custom_knowledge.json
python3 load_knowledge.py ../data/my_custom_knowledge.json
```

---

## ⚠️ Troubleshooting

### Erro: "Connection refused"
Servidor EVA-Mind não está rodando.
```bash
# Verificar se servidor está up
curl http://localhost:8080/health

# Iniciar servidor
cd ..
go run cmd/server/main.go
```

### Erro: "404 Not Found"
Endpoint `/self/teach` não encontrado. Verifique:
1. Fase F (Core Memory) está implementada?
2. Rotas registradas em `self_routes.go`?

### Erro: "command not found: jq"
`jq` não está instalado (opcional).
```bash
# macOS
brew install jq

# Ubuntu/Debian
sudo apt-get install jq

# Ou ignore - scripts funcionam sem jq
```

---

## 📚 Documentação Completa

- [eva-carga-memoria.md](../MD/SRC/eva-carga-memoria.md) - Guia completo
- [FASE_F_SUMMARY.md](../MD/SRC/FASE_F_SUMMARY.md) - Fase F documentação
- [core_memory.yaml](../configs/core_memory.yaml) - Configuração

---

## 🎉 Sucesso!

Após rodar o script, você verá:

```
✅ CARGA INICIAL COMPLETA!

📊 Resumo:
  • Memórias carregadas: 34
  • Erros encontrados: 0

EVA está viva e pronta para ajudar! 🧠⚡💜
```

**EVA agora tem conhecimento fundamental e está pronta para atender!**
