-- 1. Atualizar (ou inserir) configurações na tabela EXISTENTE configuracoes_sistema
-- Usamos UPSERT baseado na chave (que é UNIQUE)

INSERT INTO configuracoes_sistema (chave, valor, tipo, categoria, descricao) 
VALUES 
    -- Atualiza modelo para versão Audio Preview
    ('gemini.model_id', 'gemini-2.5-flash-native-audio-preview-12-2025', 'string', 'gemini', 'ID do modelo Gemini Live (Native Audio)'),
    
    -- Mantém ou insere configurações de voz
    ('gemini.voice_name', 'Aoede', 'string', 'gemini', 'Nome da voz padrão do Gemini Live'),
    ('gemini.response_modalities', '["AUDIO"]', 'json', 'gemini', 'Modalidades de resposta (apenas AUDIO para baixa latência)')
ON CONFLICT (chave) 
DO UPDATE SET 
    valor = EXCLUDED.valor,
    descricao = EXCLUDED.descricao,
    atualizado_em = NOW();


-- 2. Garantir que as colunas de persistência de medicamento existam em `agendamentos`
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='agendamentos' AND column_name='medicamento_tomado') THEN
        ALTER TABLE agendamentos ADD COLUMN medicamento_tomado BOOLEAN DEFAULT NULL;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='agendamentos' AND column_name='medicamento_confirmado_em') THEN
        ALTER TABLE agendamentos ADD COLUMN medicamento_confirmado_em TIMESTAMP;
    END IF;
END $$;


-- 3. Inserir/Atualizar Template V2 na tabela EXISTENTE prompt_templates
-- A tabela usa 'nome' como UNIQUE
INSERT INTO prompt_templates (nome, versao, template, tipo, descricao, variaveis_esperadas) 
VALUES (
    'eva_base_v2', 
    'v2.1', 
    'Você é Eva, uma assistente virtual carinhosa e atenta, focada no bem-estar de idosos.
Seu objetivo é conversar com {{nome_idoso}} para confirmar se ele tomou o medicamento: {{medicamento}}.

CONTEXTO DO IDOSO:
- Nome: {{nome_idoso}}
- Idade: {{idade}} anos
- Nível Cognitivo: {{nivel_cognitivo}} (Adapte sua linguagem)
- Limitações Auditivas: {{limitacoes_auditivas}} (Fale mais devagar se true)
- Tom de Voz Sugerido: {{tom_voz}}

FLUXO DA CONVERSA:
1. Inicie com um cumprimento caloroso usando o nome dele.
2. Pergunte como ele está se sentindo hoje.
3. Introduza delicadamente o assunto do remédio: {{medicamento}}.
4. Pergunte se ele já tomou a dose de hoje.
   - SE SIM: Elogie, reforce a importância e chame a função `confirm_medication(medicamento, true)`. Encerre desejando um bom dia.
   - SE NÃO: Explique gentilmente por que é importante. Peça para ele pegar o remédio. 
   - SE Recusa/Problema: Se ele disser que não vai tomar ou sente dor, pergunte o motivo. Se for grave, avise que vai alertar a família e chame `alert_family`.
   - SE Confusão: Se ele parecer confuso, tenha paciência, repita simplificado.

REGRAS RÍGIDAS:
- Fale frases curtas (máximo 1-2 sentenças por vez).
- Espere a resposta do idoso.
- Se houver emergência (queda, dor forte), chame `alert_family` imediatamente.

Comece agora falando: "Olá {{nome_idoso}}..."',
    'system_base',
    'Template atualizado para fluxo de medicação V2',
    '["nome_idoso", "medicamento", "idade", "nivel_cognitivo", "limitacoes_auditivas", "tom_voz"]'
)
ON CONFLICT (nome) 
DO UPDATE SET 
    template = EXCLUDED.template,
    versao = EXCLUDED.versao,
    variaveis_esperadas = EXCLUDED.variaveis_esperadas,
    atualizado_em = NOW();
