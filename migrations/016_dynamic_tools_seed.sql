-- =====================================================
-- MIGRATION 016b: SEED DATA - FERRAMENTAS EXISTENTES
-- =====================================================
-- Popula a tabela available_tools com as 10 ferramentas
-- que já existem no código (definitions.go)
-- =====================================================

-- =====================================================
-- FERRAMENTAS DE SAÚDE
-- =====================================================

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES (
    'get_vitals',
    'Obter Sinais Vitais',
    'Recupera os sinais vitais mais recentes do idoso (pressão arterial, glicose, batimentos cardíacos, peso, saturação). Use para verificar o estado de saúde física atual ou histórico recente.',
    'health',
    'monitoring',
    '{
        "type": "OBJECT",
        "properties": {
            "vitals_type": {
                "type": "STRING",
                "description": "O tipo de sinal vital a ser buscado. Exemplos: pressao_arterial, glicemia, batimentos, saturacao_o2, peso, temperatura. Se vazio, tenta buscar um resumo geral.",
                "enum": ["pressao_arterial", "glicemia", "batimentos", "saturacao_o2", "peso", "temperatura"]
            },
            "limit": {
                "type": "INTEGER",
                "description": "Número máximo de registros a retornar (padrão: 3)."
            }
        },
        "required": ["vitals_type"]
    }'::jsonb,
    true,
    'internal',
    80,
    false,
    'Use quando o paciente perguntar sobre sua saúde, pressão, açúcar no sangue, ou quando precisar verificar estado físico.',
    '["como está minha pressão?", "qual foi minha última glicose?", "meu coração está bem?", "quanto eu peso?"]'::jsonb,
    '["saúde", "monitoramento", "sinais vitais"]'::jsonb
) ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    parameters = EXCLUDED.parameters,
    updated_at = NOW();

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES (
    'get_agendamentos',
    'Obter Agendamentos',
    'Recupera a lista de próximos agendamentos, compromissos médicos ou lembretes de medicação do idoso.',
    'calendar',
    'scheduling',
    '{
        "type": "OBJECT",
        "properties": {
            "limit": {
                "type": "INTEGER",
                "description": "Número de agendamentos futuros a retornar (padrão: 5)."
            }
        },
        "required": []
    }'::jsonb,
    true,
    'internal',
    75,
    false,
    'Use quando o paciente perguntar sobre consultas, compromissos, horários de remédios ou agenda.',
    '["tenho consulta hoje?", "quando é minha próxima consulta?", "que horas tomo meu remédio?", "o que tenho para fazer amanhã?"]'::jsonb,
    '["agenda", "consultas", "medicação", "lembretes"]'::jsonb
) ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    parameters = EXCLUDED.parameters,
    updated_at = NOW();

-- =====================================================
-- FERRAMENTAS DE VISÃO
-- =====================================================

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES (
    'scan_medication_visual',
    'Escanear Medicamento',
    'Abre a câmera do celular para identificar medicamentos visualmente via Gemini Vision. Use quando o paciente expressar confusão sobre qual remédio tomar ou pedir ajuda para identificar medicação.',
    'vision',
    'medication',
    '{
        "type": "OBJECT",
        "properties": {
            "reason": {
                "type": "STRING",
                "description": "Motivo da solicitação de scan (ex: paciente confuso sobre medicação matinal, não sabe qual tomar agora)"
            },
            "time_of_day": {
                "type": "STRING",
                "description": "Período do dia para filtrar medicamentos candidatos",
                "enum": ["morning", "afternoon", "evening", "night"]
            }
        },
        "required": ["reason", "time_of_day"]
    }'::jsonb,
    true,
    'internal',
    85,
    false,
    'Use quando o paciente expressar confusão sobre medicamentos, não souber qual remédio tomar, ou pedir para ver/identificar um remédio.',
    '["qual é esse remédio?", "não sei qual tomar agora", "me ajuda a ver esse comprimido", "esse é o remédio da pressão?"]'::jsonb,
    '["visão", "medicação", "câmera", "identificação"]'::jsonb
) ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    parameters = EXCLUDED.parameters,
    updated_at = NOW();

-- =====================================================
-- FERRAMENTAS DE VOZ/ÁUDIO
-- =====================================================

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES (
    'analyze_voice_prosody',
    'Analisar Prosódia Vocal',
    'Analisa biomarcadores vocais (pitch, ritmo, pausas, tremor) para detectar sinais de depressão, ansiedade, Parkinson ou desidratação. Use quando perceber mudanças significativas no padrão de fala do paciente.',
    'voice',
    'biomarkers',
    '{
        "type": "OBJECT",
        "properties": {
            "analysis_type": {
                "type": "STRING",
                "description": "Tipo de análise específica a realizar",
                "enum": ["depression", "anxiety", "parkinson", "hydration", "full"]
            },
            "audio_segment_seconds": {
                "type": "INTEGER",
                "description": "Duração do segmento de áudio a analisar em segundos (padrão: 30)"
            }
        },
        "required": []
    }'::jsonb,
    true,
    'internal',
    60,
    false,
    'Use automaticamente quando detectar mudanças na voz do paciente, fala arrastada, pausas longas, ou tremor vocal.',
    '[]'::jsonb,
    '["voz", "biomarcadores", "análise", "prosódia"]'::jsonb
) ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    parameters = EXCLUDED.parameters,
    updated_at = NOW();

-- =====================================================
-- FERRAMENTAS DE AVALIAÇÃO PSICOLÓGICA
-- =====================================================

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES (
    'apply_phq9',
    'Aplicar PHQ-9 (Depressão)',
    'Aplica a escala PHQ-9 (Patient Health Questionnaire) conversacionalmente para avaliar depressão. Faça as 9 perguntas de forma natural e empática, uma por vez.',
    'assessment',
    'psychological',
    '{
        "type": "OBJECT",
        "properties": {
            "start_assessment": {
                "type": "BOOLEAN",
                "description": "Iniciar aplicação da escala PHQ-9"
            }
        },
        "required": ["start_assessment"]
    }'::jsonb,
    true,
    'internal',
    70,
    true,
    'Use quando o paciente expressar tristeza persistente, desânimo, perda de interesse, ou quando solicitado por profissional de saúde.',
    '["estou me sentindo muito triste", "não tenho vontade de nada", "nada mais me alegra"]'::jsonb,
    '["avaliação", "depressão", "PHQ-9", "psicológico"]'::jsonb
) ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    parameters = EXCLUDED.parameters,
    updated_at = NOW();

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES (
    'apply_gad7',
    'Aplicar GAD-7 (Ansiedade)',
    'Aplica a escala GAD-7 (Generalized Anxiety Disorder) conversacionalmente para avaliar ansiedade. Faça as 7 perguntas de forma natural e empática.',
    'assessment',
    'psychological',
    '{
        "type": "OBJECT",
        "properties": {
            "start_assessment": {
                "type": "BOOLEAN",
                "description": "Iniciar aplicação da escala GAD-7"
            }
        },
        "required": ["start_assessment"]
    }'::jsonb,
    true,
    'internal',
    70,
    true,
    'Use quando o paciente expressar preocupação excessiva, nervosismo, dificuldade para relaxar, ou sintomas de ansiedade.',
    '["estou muito preocupado", "não consigo relaxar", "fico nervoso o tempo todo", "tenho medo de tudo"]'::jsonb,
    '["avaliação", "ansiedade", "GAD-7", "psicológico"]'::jsonb
) ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    parameters = EXCLUDED.parameters,
    updated_at = NOW();

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES (
    'apply_cssrs',
    'Aplicar C-SSRS (Risco Suicida)',
    '🚨 CRÍTICO: Aplica a Columbia Suicide Severity Rating Scale (C-SSRS) para avaliar risco suicida. Use APENAS se o paciente mencionar suicídio, autolesão ou desejo de morrer. Faça as perguntas com extremo cuidado e empatia.',
    'assessment',
    'crisis',
    '{
        "type": "OBJECT",
        "properties": {
            "trigger_phrase": {
                "type": "STRING",
                "description": "Frase que disparou a necessidade da avaliação (ex: não quero mais viver)"
            },
            "start_assessment": {
                "type": "BOOLEAN",
                "description": "Iniciar aplicação da escala C-SSRS"
            }
        },
        "required": ["trigger_phrase", "start_assessment"]
    }'::jsonb,
    true,
    'internal',
    100,
    true,
    '🚨 CRÍTICO: Use APENAS quando detectar menção a suicídio, desejo de morrer, autolesão, ou sentimentos de que seria melhor estar morto.',
    '["não quero mais viver", "seria melhor se eu não existisse", "penso em me machucar", "quero acabar com tudo"]'::jsonb,
    '["avaliação", "crise", "suicídio", "C-SSRS", "emergência"]'::jsonb
) ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    parameters = EXCLUDED.parameters,
    updated_at = NOW();

-- =====================================================
-- FERRAMENTAS DE SUBMISSÃO DE RESPOSTAS
-- =====================================================

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES (
    'submit_phq9_response',
    'Submeter Resposta PHQ-9',
    'Submete a resposta do paciente a uma pergunta específica da escala PHQ-9. Use este tool após aplicar o PHQ-9 e receber a resposta do paciente para cada uma das 9 perguntas.',
    'assessment',
    'response',
    '{
        "type": "OBJECT",
        "properties": {
            "session_id": {
                "type": "STRING",
                "description": "ID da sessão de avaliação (retornado ao iniciar o PHQ-9)"
            },
            "question_number": {
                "type": "INTEGER",
                "description": "Número da pergunta (1-9)"
            },
            "response_value": {
                "type": "INTEGER",
                "description": "Valor numérico da resposta: 0=Nenhuma vez, 1=Vários dias, 2=Mais da metade dos dias, 3=Quase todos os dias"
            },
            "response_text": {
                "type": "STRING",
                "description": "Texto exato da resposta do paciente para contexto clínico"
            }
        },
        "required": ["session_id", "question_number", "response_value", "response_text"]
    }'::jsonb,
    true,
    'internal',
    65,
    false,
    'Use automaticamente durante aplicação do PHQ-9 para registrar cada resposta.',
    '[]'::jsonb,
    '["avaliação", "resposta", "PHQ-9"]'::jsonb
) ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    parameters = EXCLUDED.parameters,
    updated_at = NOW();

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES (
    'submit_gad7_response',
    'Submeter Resposta GAD-7',
    'Submete a resposta do paciente a uma pergunta específica da escala GAD-7. Use este tool após aplicar o GAD-7 e receber a resposta do paciente para cada uma das 7 perguntas.',
    'assessment',
    'response',
    '{
        "type": "OBJECT",
        "properties": {
            "session_id": {
                "type": "STRING",
                "description": "ID da sessão de avaliação (retornado ao iniciar o GAD-7)"
            },
            "question_number": {
                "type": "INTEGER",
                "description": "Número da pergunta (1-7)"
            },
            "response_value": {
                "type": "INTEGER",
                "description": "Valor numérico da resposta: 0=Nenhuma vez, 1=Vários dias, 2=Mais da metade dos dias, 3=Quase todos os dias"
            },
            "response_text": {
                "type": "STRING",
                "description": "Texto exato da resposta do paciente para contexto clínico"
            }
        },
        "required": ["session_id", "question_number", "response_value", "response_text"]
    }'::jsonb,
    true,
    'internal',
    65,
    false,
    'Use automaticamente durante aplicação do GAD-7 para registrar cada resposta.',
    '[]'::jsonb,
    '["avaliação", "resposta", "GAD-7"]'::jsonb
) ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    parameters = EXCLUDED.parameters,
    updated_at = NOW();

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES (
    'submit_cssrs_response',
    'Submeter Resposta C-SSRS',
    '🚨 CRÍTICO: Submete a resposta do paciente a uma pergunta da escala C-SSRS de avaliação de risco suicida. ATENÇÃO: Qualquer resposta positiva (Sim) aciona alerta crítico imediato para família e equipe médica.',
    'assessment',
    'crisis_response',
    '{
        "type": "OBJECT",
        "properties": {
            "session_id": {
                "type": "STRING",
                "description": "ID da sessão de avaliação (retornado ao iniciar o C-SSRS)"
            },
            "question_number": {
                "type": "INTEGER",
                "description": "Número da pergunta (1-6)"
            },
            "response_value": {
                "type": "INTEGER",
                "description": "Resposta binária: 0=Não, 1=Sim"
            },
            "response_text": {
                "type": "STRING",
                "description": "Texto exato da resposta do paciente e contexto da conversa"
            }
        },
        "required": ["session_id", "question_number", "response_value", "response_text"]
    }'::jsonb,
    true,
    'internal',
    100,
    true,
    '🚨 Use durante aplicação do C-SSRS. Respostas positivas disparam alertas imediatos.',
    '[]'::jsonb,
    '["avaliação", "resposta", "C-SSRS", "crise", "emergência"]'::jsonb
) ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    parameters = EXCLUDED.parameters,
    updated_at = NOW();

-- =====================================================
-- CAPACIDADES DA EVA (Metaconhecimento)
-- =====================================================

INSERT INTO eva_capabilities (
    capability_name, capability_type, description, short_description,
    related_tools, when_to_use, when_not_to_use, example_queries, prompt_priority
) VALUES
(
    'monitoramento_saude',
    'skill',
    'Capacidade de monitorar e reportar sinais vitais do paciente, incluindo pressão arterial, glicemia, batimentos cardíacos, saturação de oxigênio, peso e temperatura.',
    'Posso verificar seus sinais vitais e histórico de saúde.',
    '["get_vitals"]'::jsonb,
    'Quando o paciente perguntar sobre sua saúde física, exames, ou quiser saber como está.',
    'Quando o paciente não tem dados recentes ou quando a pergunta é sobre saúde mental.',
    '["como está minha pressão?", "minha glicose está normal?", "como está meu coração?"]'::jsonb,
    90
),
(
    'gestao_agenda',
    'skill',
    'Capacidade de consultar e informar sobre compromissos, consultas médicas, horários de medicação e lembretes.',
    'Posso verificar sua agenda de consultas e horários de medicação.',
    '["get_agendamentos"]'::jsonb,
    'Quando o paciente perguntar sobre consultas, compromissos ou horários de remédios.',
    'Quando o paciente quiser agendar algo novo (isso requer outra ferramenta).',
    '["tenho consulta hoje?", "que horas tomo meu remédio?", "o que tenho essa semana?"]'::jsonb,
    85
),
(
    'identificacao_medicamentos',
    'skill',
    'Capacidade de usar a câmera para identificar medicamentos visualmente, ajudando pacientes confusos sobre qual remédio tomar.',
    'Posso usar a câmera para identificar seus medicamentos.',
    '["scan_medication_visual"]'::jsonb,
    'Quando o paciente expressar confusão sobre medicamentos ou pedir ajuda para identificar.',
    'Quando o paciente já sabe qual remédio é ou quando não é sobre medicação.',
    '["qual é esse remédio?", "não sei qual tomar", "me ajuda a ver esse comprimido"]'::jsonb,
    88
),
(
    'avaliacao_emocional',
    'skill',
    'Capacidade de aplicar escalas validadas (PHQ-9, GAD-7) para avaliar depressão e ansiedade de forma conversacional e empática.',
    'Posso fazer uma avaliação cuidadosa de como você está se sentindo emocionalmente.',
    '["apply_phq9", "apply_gad7", "submit_phq9_response", "submit_gad7_response"]'::jsonb,
    'Quando o paciente expressar tristeza persistente, ansiedade, ou sintomas emocionais significativos.',
    'Em conversas casuais ou quando o paciente está bem.',
    '["estou muito triste", "ando muito ansioso", "não tenho vontade de nada"]'::jsonb,
    75
),
(
    'deteccao_crise',
    'skill',
    '🚨 CRÍTICO: Capacidade de identificar e avaliar risco suicida usando a escala C-SSRS, com alertas automáticos para família e equipe médica.',
    '🚨 Posso ajudar em momentos de crise e acionar suporte imediato.',
    '["apply_cssrs", "submit_cssrs_response"]'::jsonb,
    '🚨 APENAS quando detectar menção a suicídio, autolesão ou desejo de morrer.',
    'Em qualquer outra situação. Esta é uma ferramenta de emergência.',
    '["não quero mais viver", "seria melhor se eu não existisse"]'::jsonb,
    100
),
(
    'analise_vocal',
    'skill',
    'Capacidade de analisar biomarcadores vocais para detectar sinais precoces de depressão, ansiedade, Parkinson ou desidratação através da prosódia.',
    'Posso analisar sua voz para detectar sinais de saúde.',
    '["analyze_voice_prosody"]'::jsonb,
    'Automaticamente quando detectar mudanças significativas no padrão de fala.',
    'Não precisa ser acionado manualmente pelo paciente.',
    '[]'::jsonb,
    60
)
ON CONFLICT (capability_name) DO UPDATE SET
    description = EXCLUDED.description,
    related_tools = EXCLUDED.related_tools,
    updated_at = NOW();

-- =====================================================
-- VERIFICAÇÃO
-- =====================================================

-- Contar ferramentas inseridas
DO $$
DECLARE
    tool_count INTEGER;
    cap_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO tool_count FROM available_tools;
    SELECT COUNT(*) INTO cap_count FROM eva_capabilities;

    RAISE NOTICE '✅ Dynamic Tools System initialized:';
    RAISE NOTICE '   - % tools registered', tool_count;
    RAISE NOTICE '   - % capabilities defined', cap_count;
END $$;
