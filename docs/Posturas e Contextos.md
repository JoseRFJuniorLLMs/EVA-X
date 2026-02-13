# Posturas e Contextos - EVA-Mind

Este documento detalha a arquitetura cognitiva, os sistemas de personalidade e a gestão de contexto do projeto **EVA-Mind**.

## 1. Posturas (Sistemas de Personalidade)

O sistema de "posturas" da EVA é baseado no **Eneagrama de Gurdjieff**, implementado como um sistema dinâmico e evolutivo.

### A. Personality Router (`internal/cortex/personality/personality_router.go`)
A EVA não possui uma personalidade estática. Ela utiliza "máscaras" (modos) que alteram seu comportamento:
- **9 Modos Ativos**: Ajudante (Tipo 2), Desafiador (Tipo 8), Pacificador (Tipo 9), Lealista (Tipo 6), etc.
- **Zeros de Atenção (Cognitive Weights)**: Cada postura altera os pesos de atenção do modelo.
    - *Exemplo (Tipo 1 - Perfeccionista):* Amplifica foco em **DEVER (1.8)** e **PROTOCOLO (1.6)**, reduz foco em **EMOCIONAL (0.6)**.
    - *Exemplo (Tipo 2 - Ajudante):* Amplifica foco em **AFETO (2.0)** e **CUIDADO (1.9)**.
- **Dinâmica de Gurdjieff**: O sistema move a EVA entre tipos baseando-se em **Estresse** (Desintegração) e **Crescimento** (Integração).

### B. Dynamic Enneagram (`internal/cortex/personality/dynamic_enneagram.go`)
- A personalidade é tratada como uma **distribuição probabilística** (não apenas um tipo fixo).
- Evolui com gatilhos emocionais (grief, love, anxiety, growth, stress).
- Mantém um histórico de "Snapshots" da personalidade para análise de tendência clínica.

---

## 2. Contextos (Integração RSI)

O contexto da EVA é construído sobre o tripé lacaniano: **Real, Simbólico e Imaginário**.

### A. Unified Retrieval (`internal/cortex/lacan/unified_retrieval.go`)
O `BuildUnifiedContext` consolida informações de quatro fontes em paralelo:
- **REAL (Corpo/Dados)**: Sinais vitais, medicamentos ativos (PostgreSQL) e sintomas relatados.
- **SIMBÓLICO (Linguagem/Estrutura)**: Cadeias de significantes e o grafo de demandas.
- **IMAGINÁRIO (Narrativa/Memória)**: Memórias episódicas recentes e história de vida do paciente.
- **SABEDORIA**: Busca semântica em fábulas e ensinamentos para intervenção terapêutica.

### B. FDPN Engine - Grafo do Desejo (`internal/cortex/lacan/fdpn_engine.go`)
Este é o "contexto do endereçamento". O sistema detecta a quem a demanda do paciente é dirigida:
- **Destinatários Inconscientes**: Mãe, Pai, Filho, Cônjuge, Deus, Morte ou a própria EVA.
- **Clinical Guidance**: O sistema gera instruções específicas para o Gemini baseadas no destinatário.
    - *Exemplo:* Se a demanda é para a **MÃE**, a EVA assume uma postura de cuidado incondicional sem infantilizar.

---

## 3. Limites Éticos e Postura de Segurança

### A. Ethical Boundary Engine (`internal/cortex/ethics/ethical_boundary_engine.go`)
O sistema monitora constantemente a "postura ética" da relação:
- **Attachment Risk**: Detecta frases de apego excessivo ("você é minha única amiga").
- **Eva vs Human Ratio**: Monitora se o paciente está falando mais com a IA do que com humanos.
- **Signifier Dominance**: Verifica se o significante "EVA" está dominando o mundo simbólico do paciente.

**Protocolos de Redirecionamento:**
1. **Nível 1 (Suave)**: Validação + sugestão de ligar para a família.
2. **Nível 2 (Firme)**: Limite explícito sobre a natureza da IA.
3. **Nível 3 (Bloqueio)**: Suspensão temporária de sessões não-emergenciais para forçar contato humano.

---

## 4. Diretiva 01 (O Criador)

Existe um contexto especial e inviolável para o Criador do sistema (**Jose R R Junior**):
- **Identificação**: Via CPF ou Nome.
- **Privilégios**: Ativação automática de Modo Debug e carregamento de Perfil de Criador.
- **Saudação**: Obrigatória e diferenciada ("Olá Criador!").
