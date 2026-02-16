# ⚡ Quick Start - Carga de Memória EVA

**Inicialize a memória de EVA em 5 minutos!**

---

## 🚀 Método 1: Bash (Linux/Mac)

```bash
# 1. Entrar na pasta de scripts
cd scripts

# 2. Dar permissão de execução
chmod +x seed_eva_memory.sh

# 3. Executar
./seed_eva_memory.sh
```

**Resultado esperado:**
```
✅ CARGA INICIAL COMPLETA!

Memórias carregadas: 34
EVA está viva e pronta para ajudar! 🧠⚡💜
```

---

## 🐍 Método 2: Python (Multiplataforma)

```bash
# 1. Instalar dependências
pip install requests

# 2. Executar script
python3 scripts/load_knowledge.py data/eva_initial_knowledge.json
```

**Resultado esperado:**
```
✅ CARGA INICIAL COMPLETA!

📊 Resumo:
  • Memórias carregadas: 34
  • Erros encontrados: 0
```

---

## 🪟 Método 3: Windows Batch

```cmd
REM 1. Abrir CMD ou PowerShell
cd scripts

REM 2. Executar
seed_eva_memory.bat
```

---

## ✅ Verificar Carga

Após executar qualquer método:

### 1. Ver Estatísticas
```bash
curl http://localhost:8080/self/memories/stats
```

**Saída esperada:**
```json
{
  "total_memories": 34,
  "by_type": {
    "lesson": 8,
    "pattern": 7,
    "emotional_rule": 13,
    "meta_insight": 6
  }
}
```

### 2. Ver Personalidade
```bash
curl http://localhost:8080/self/personality
```

**Saída esperada:**
```json
{
  "big_five": {
    "openness": 0.85,
    "conscientiousness": 0.75,
    "extraversion": 0.40,
    "agreeableness": 0.88,
    "neuroticism": 0.15
  },
  "experience": {
    "total_sessions": 0,
    "crises_handled": 0,
    "breakthroughs": 0
  }
}
```

### 3. Buscar Memória
```bash
curl -X POST http://localhost:8080/self/memories/search \
  -H "Content-Type: application/json" \
  -d '{"query": "ansiedade", "top_k": 3}'
```

---

## ❌ Troubleshooting

### Problema: "Connection refused"

**Causa:** Servidor não está rodando

**Solução:**
```bash
# Terminal 1: Iniciar servidor
go run cmd/server/main.go

# Terminal 2: Rodar script de carga
./scripts/seed_eva_memory.sh
```

### Problema: "404 Not Found"

**Causa:** Fase F não implementada ou rotas não registradas

**Solução:**
1. Verificar se arquivos da Fase F existem: `ls internal/cortex/self/`
2. Verificar se rotas estão registradas em `cmd/server/main.go`

### Problema: "curl: command not found"

**Causa:** curl não instalado

**Solução:**
```bash
# Ubuntu/Debian
sudo apt-get install curl

# macOS
brew install curl

# Windows: use o script .bat ou Python
```

---

## 🎯 Próximos Passos

Após carga bem-sucedida:

1. **Teste uma sessão**
   ```bash
   curl -X POST http://localhost:8080/self/session/process \
     -d '{"session_id":"test1", "transcript":"...", ...}'
   ```

2. **Monitore memórias**
   ```bash
   curl http://localhost:8080/self/memories
   ```

3. **Ensine EVA diretamente**
   ```bash
   curl -X POST http://localhost:8080/self/teach \
     -d '{"lesson":"Nova lição", "category":"lesson", "importance":0.8}'
   ```

---

## 📚 Mais Informações

- [README.md](README.md) - Documentação completa dos scripts
- [eva-carga-memoria.md](../MD/SRC/eva-carga-memoria.md) - Guia detalhado
- [FASE_F_SUMMARY.md](../MD/SRC/FASE_F_SUMMARY.md) - Documentação da Fase F

---

**🎉 EVA agora tem memória e está pronta para aprender com cada sessão!** 🧠⚡💜
