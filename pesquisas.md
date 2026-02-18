# 📊 Registros de Acesso e Uso no EVA-Mind

Sim, **existe um sistema completo de auditoria e logs** no PostgreSQL do EVA-Mind. Aqui está tudo o que é registrado:

---

## 1️⃣ LGPD Audit Trail (Lei Geral de Proteção de Dados)

**Tabela:** `lgpd_audit_log`

Registra **quem** acessou **quais dados** de **qual idoso**:
- `actor_id` — Quem fez a ação (usuário, sistema, cuidador, admin)
- `actor_type` — Tipo de ator
- `actor_ip` — IP de quem acessou
- `subject_id` — Qual idoso foi afetado
- `event_type` — Tipo de evento (access, update, delete, etc)
- `action` — Ação específica realizada
- `timestamp` — Quando aconteceu
- `success` — Se funcionou ou não
- `error_message` — Se falhou, qual foi o erro
- **Retenção:** 365 dias (configurável)

### Exemplo de Query:
```sql
SELECT * FROM lgpd_audit_log
WHERE subject_id = 123
ORDER BY timestamp DESC
LIMIT 50;
```

---

## 2️⃣ Histórico de Chamadas (Conversas)

**Tabela:** `episodic_memories`

Registra **cada conversa completa** entre idoso e EVA:
- `idoso_id` — Qual idoso
- `speaker` — Quem falou ("user" ou "assistant")
- `content` — O que foi dito (texto completo)
- `emotion` — Emoção detectada
- `importance` — Score de importância (0.0-1.0)
- `timestamp` — Quando aconteceu
- `session_id` — Qual sessão de conversa
- `is_atomic` — Se foi decomposto em fatos atômicos

### Exemplos de Query:
```sql
-- Todas as conversas de um idoso específico
SELECT timestamp, speaker, content, emotion, importance
FROM episodic_memories
WHERE idoso_id = 123
ORDER BY timestamp DESC;

-- Resumo de atividade por dia
SELECT DATE(timestamp) as data, COUNT(*) as total_mensagens
FROM episodic_memories
WHERE idoso_id = 123
GROUP BY DATE(timestamp)
ORDER BY data DESC;
```

---

## 3️⃣ Histórico de Ligações (Chamadas Telefônicas)

**Tabela:** `historico_ligacoes`

Registra **cada chamada** realizada:
- `idoso_id` — Qual idoso
- `call_sid` — ID único da chamada
- `status` — Status da chamada
- `inicio` — Quando começou
- `fim` — Quando terminou
- `qualidade_audio` — Qualidade do áudio
- `interrupcoes_detectadas` — Número de interrupções

### Exemplo de Query:
```sql
SELECT * FROM historico_ligacoes
WHERE idoso_id = 123
ORDER BY inicio DESC
LIMIT 20;
```

---

## 4️⃣ Logs de Escalonamento (Alertas de Emergência)

**Tabela:** `escalation_logs`

Registra **cada alerta/emergência** disparado:
- `elder_name` — Qual idoso
- `reason` — Por que foi disparado
- `priority` — Nível de criticidade
- `acknowledged` — Se foi reconhecido
- `acknowledged_by` — Quem reconheceu
- `final_channel` — Como foi entregue (push, SMS, WhatsApp, etc)
- `started_at` — Quando começou
- `completed_at` — Quando terminou

### Exemplo de Query:
```sql
SELECT * FROM escalation_logs
WHERE elder_name LIKE 'Nome%'
ORDER BY started_at DESC;
```

---

## 5️⃣ Consents & Consentimentos (LGPD)

**Tabela:** `lgpd_consents`

Registra **consentimentos explícitos** para cada tipo de uso:
- `subject_id` — Qual idoso
- `consent_type` — Tipo (general, health_data, research, marketing)
- `granted` — Se foi concedido ou não
- `granted_at` — Quando foi concedido
- `revoked_at` — Se foi revogado

### Exemplo de Query:
```sql
SELECT * FROM lgpd_consents
WHERE subject_id = 123;
```

---

## 6️⃣ Data Requests (Direito de Acesso - Art. 18 LGPD)

**Tabela:** `lgpd_requests`

Registra **pedidos de acesso aos dados** do idoso:
- `request_type` — Tipo (access, deletion, rectification, portability, etc)
- `status` — Status do pedido
- `requested_at` — Quando foi solicitado
- `deadline_at` — Prazo para responder (15 dias)
- `handled_by` — Quem tratou
- `response_details` — Detalhes da resposta

### Exemplo de Query:
```sql
SELECT * FROM lgpd_requests
WHERE subject_id = 123
ORDER BY requested_at DESC;
```

---

## 7️⃣ Cleanup Automático de Logs Antigos

Função PostgreSQL para limpeza:
```sql
SELECT cleanup_expired_audit_logs();
-- Remove automaticamente logs com mais de 365 dias
```

---

## 🔍 Exemplo Completo: Verificar Quem Usou o Sistema

### Todos os acessos da semana passada
```sql
SELECT
  lgpd_audit_log.timestamp,
  lgpd_audit_log.actor_id,
  lgpd_audit_log.actor_type,
  lgpd_audit_log.action,
  idosos.nome as "idoso_afetado"
FROM lgpd_audit_log
JOIN idosos ON lgpd_audit_log.subject_id = idosos.id
WHERE lgpd_audit_log.timestamp > NOW() - INTERVAL '7 days'
ORDER BY lgpd_audit_log.timestamp DESC;
```

### Atividade de um idoso específico
```sql
SELECT
  DATE(episodic_memories.timestamp) as "data",
  COUNT(*) as "mensagens_trocadas",
  COUNT(DISTINCT episodic_memories.session_id) as "sessoes"
FROM episodic_memories
WHERE episodic_memories.idoso_id = 123
GROUP BY DATE(episodic_memories.timestamp)
ORDER BY data DESC;
```

### Resumo de alertas por prioridade
```sql
SELECT
  priority,
  COUNT(*) as total,
  SUM(CASE WHEN acknowledged THEN 1 ELSE 0 END) as reconhecidos,
  AVG(EXTRACT(EPOCH FROM (completed_at - started_at)))::INTEGER as tempo_medio_segundos
FROM escalation_logs
WHERE started_at > NOW() - INTERVAL '30 days'
GROUP BY priority
ORDER BY priority;
```

---

## ⚠️ Dados Sensíveis Protegidos

- CPF é **hashado** na auditoria (`subject_cpf`)
- Embeddings **não são armazenados** em logs de auditoria
- Acesso requer **autenticação e autorização**
- Sistema segue **LGPD (Lei Geral de Proteção de Dados)**
- Retenção automática: Logs antigos são deletados após 365 dias
- Cada tabela possui **índices otimizados** para queries rápidas

---

## 📄 Arquivo de Origem

Estes dados foram extraídos da migration:
- `migrations/018_lgpd_audit_trail.sql` — Sistema completo de auditoria LGPD

---

## 🎯 Resumo

O EVA-Mind tem um sistema **enterprise-grade** de auditoria com rastreamento completo de:
- ✅ Quem usou o sistema
- ✅ Quando acessou
- ✅ Qual idoso foi afetado
- ✅ Que tipo de operação foi feita
- ✅ Se funcionou ou deu erro
- ✅ IP de origem
- ✅ Conformidade LGPD

Todos os registros são **criptografados**, **auditados** e **retenção automática** é aplicada. 🔐
