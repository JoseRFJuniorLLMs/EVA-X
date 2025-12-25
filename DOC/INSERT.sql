-- 1. Insere o idoso respeitando todas as restrições do seu schema (eva-v7.sql)
INSERT INTO idosos (
    nome, 
    data_nascimento, 
    telefone, 
    nivel_cognitivo, 
    limitacoes_auditivas, 
    tom_voz,
    timezone,
    familiar_principal
) 
VALUES (
    'JOSE R F JUNIOR', 
    '1980-01-01', 
    '+351966805210', 
    'normal', 
    false, 
    'amigavel', 
    'Europe/Lisbon',
    '{"nome": "Contato Teste", "telefone": "+351966805210", "parentesco": "filho"}'
);

-- 2. Insere o agendamento usando as colunas corretas (tipo, data_hora_agendada, dados_tarefa)
INSERT INTO agendamentos (
    idoso_id, 
    tipo, 
    data_hora_agendada, 
    status, 
    prioridade,
    dados_tarefa
) 
SELECT 
    id, 
    'lembrete_medicamento', -- Tipo obrigatório no seu schema
    NOW(), 
    'agendado', 
    'normal',
    '{"medicamento": "Café e Teste EVA-Mind", "dosagem": "1 xícara", "horario_previsto": "agora"}'
FROM idosos 
WHERE nome = 'JOSE R F JUNIOR' 
ORDER BY id DESC 
LIMIT 1;

UPDATE agendamentos 
SET status = 'agendado', 
    data_hora_agendada = NOW() - INTERVAL '5 minutes'
WHERE idoso_id = (SELECT id FROM idosos WHERE nome = 'JOSE R F JUNIOR' ORDER BY id DESC LIMIT 1)
  AND status != 'concluido';