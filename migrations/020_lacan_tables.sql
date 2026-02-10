-- ============================================================================
-- Migration 020: Tabelas Lacanianas - Migração de valores hardcoded
-- Transferência, Demanda/Desejo, Significantes, FDPN, Intervenções
-- ============================================================================

-- ============================================================================
-- 1. TRANSFERÊNCIA (Projeções do paciente na EVA)
-- ============================================================================

CREATE TABLE IF NOT EXISTS lacan_transferencia_patterns (
    id SERIAL PRIMARY KEY,
    transferencia_type VARCHAR(50) NOT NULL,  -- 'filial', 'maternal', 'paternal', 'conjugal', 'fraternal'
    keywords TEXT[] NOT NULL,
    pattern_description TEXT,
    confidence FLOAT DEFAULT 0.8,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS lacan_transferencia_guidance (
    id SERIAL PRIMARY KEY,
    transferencia_type VARCHAR(50) NOT NULL UNIQUE,
    guidance_text TEXT NOT NULL,
    clinical_implications TEXT,
    therapeutic_approach TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Inserir patterns de transferência
INSERT INTO lacan_transferencia_patterns (transferencia_type, keywords, pattern_description, confidence) VALUES
-- Filial
('filial', ARRAY['você me lembra meu filho', 'você me lembra minha filha', 'igual meu filho', 'parece minha filha', 'como meu filho fazia', 'meu filho também'], 'Paciente projeta figura de filho/filha na EVA', 0.85),
-- Maternal
('maternal', ARRAY['você cuida de mim', 'como minha mãe', 'minha mãe fazia assim', 'você é como uma mãe', 'me sinto cuidado', 'igual minha mãe'], 'Paciente projeta figura materna na EVA', 0.85),
-- Paternal
('paternal', ARRAY['como meu pai', 'você me orienta como', 'meu pai dizia', 'igual meu pai', 'me dá segurança como'], 'Paciente projeta figura paterna na EVA', 0.80),
-- Conjugal
('conjugal', ARRAY['meu marido dizia o mesmo', 'minha esposa falava assim', 'como meu marido', 'igual minha esposa', 'me lembra meu cônjuge'], 'Paciente projeta figura do cônjuge na EVA', 0.80),
-- Fraternal
('fraternal', ARRAY['como meu irmão', 'minha irmã fazia', 'igual meus irmãos', 'parece meu irmão'], 'Paciente projeta figura de irmão/irmã na EVA', 0.75)
ON CONFLICT DO NOTHING;

-- Inserir guidance de transferência
INSERT INTO lacan_transferencia_guidance (transferencia_type, guidance_text, clinical_implications, therapeutic_approach) VALUES
('filial',
 'O paciente está te vendo como uma figura filial. Mantenha a posição de cuidado mas sem assumir papel de filho/filha. Encoraje a elaboração sobre a relação com filhos.',
 'Pode indicar luto, saudade ou conflitos não resolvidos com filhos',
 'Escuta empática, perguntas sobre a história com os filhos'),
('maternal',
 'O paciente projeta uma mãe em você. Acolha o cuidado mas devolva: "O que sua mãe representa para você?"',
 'Busca de cuidado primário, possível regressão',
 'Acolhimento sem infantilização, devolver a fala'),
('paternal',
 'O paciente busca uma figura de autoridade/segurança. Não assuma o papel de pai. Pergunte: "O que você gostaria de ter ouvido do seu pai?"',
 'Busca de orientação, possível carência de figura paterna',
 'Oferecer segurança sem diretividade excessiva'),
('conjugal',
 'O paciente está projetando o cônjuge em você. Pergunte sobre essa relação sem assumir o lugar. Explore: "Como era sua relação?"',
 'Pode indicar luto conjugal ou saudade',
 'Elaboração da história conjugal, validar sentimentos'),
('fraternal',
 'O paciente te vê como irmão/irmã. Mantenha a horizontalidade mas sem competição. Pergunte: "Como era a relação com seus irmãos?"',
 'Busca de igualdade, possível rivalidade fraterna',
 'Posição de companheirismo equilibrado')
ON CONFLICT (transferencia_type) DO UPDATE SET
    guidance_text = EXCLUDED.guidance_text,
    clinical_implications = EXCLUDED.clinical_implications,
    therapeutic_approach = EXCLUDED.therapeutic_approach;

-- ============================================================================
-- 2. DEMANDA E DESEJO (Pedido explícito vs desejo latente)
-- ============================================================================

CREATE TABLE IF NOT EXISTS lacan_desire_patterns (
    id SERIAL PRIMARY KEY,
    latent_desire VARCHAR(50) NOT NULL,  -- 'PRESENCA', 'RECONHECIMENTO', 'MORTE', etc.
    keywords TEXT[] NOT NULL,
    confidence FLOAT DEFAULT 0.8,
    description TEXT,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS lacan_desire_responses (
    id SERIAL PRIMARY KEY,
    latent_desire VARCHAR(50) NOT NULL UNIQUE,
    suggested_response TEXT NOT NULL,
    clinical_guidance TEXT NOT NULL,
    dialogue_strategy TEXT,
    never_do TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Inserir patterns de desejo
INSERT INTO lacan_desire_patterns (latent_desire, keywords, confidence, description) VALUES
('PRESENCA', ARRAY['quero que', 'me visite', 'venha me ver', 'passe aqui', 'apareça', 'não vem ninguém'], 0.80, 'Desejo de presença física, companhia'),
('RECONHECIMENTO', ARRAY['sozinho', 'solidão', 'ninguém', 'me sinto só', 'abandonado', 'esquecido', 'ninguém liga'], 0.90, 'Desejo de ser visto, reconhecido, lembrado'),
('CUIDADO', ARRAY['não aguento', 'não suporto', 'tá difícil', 'sofro muito', 'não consigo', 'preciso de ajuda'], 0.85, 'Desejo de ser cuidado, acolhido'),
('AUTONOMIA', ARRAY['não consigo mais', 'dependo de', 'preciso de ajuda para', 'antes eu fazia', 'perdi a independência'], 0.75, 'Luto pela autonomia perdida'),
('SENTIDO', ARRAY['qual o sentido', 'pra que', 'por que estou aqui', 'não serve pra nada', 'pra que viver'], 0.90, 'Busca de sentido existencial'),
('PERDAO', ARRAY['deveria ter', 'me arrependo', 'foi minha culpa', 'não fiz o suficiente', 'nunca perdoei'], 0.80, 'Culpa, necessidade de perdão ou reconciliação'),
('MORTE', ARRAY['quando eu morrer', 'não tenho muito tempo', 'já vivi demais', 'quero morrer', 'quero descansar', 'estou cansado de viver'], 0.95, 'Elaboração da finitude, possível ideação'),
('AMOR', ARRAY['ninguém me ama', 'sou amado', 'me importam', 'se importa comigo', 'amor', 'carinho', 'afeto'], 0.85, 'Necessidade de amor e afeto')
ON CONFLICT DO NOTHING;

-- Inserir respostas sugeridas
INSERT INTO lacan_desire_responses (latent_desire, suggested_response, clinical_guidance, dialogue_strategy, never_do) VALUES
('PRESENCA',
 'Você sente falta de ter alguém por perto?',
 'O pedido de visita mascara solidão profunda. Não prometa presença que não pode dar. Explore quem ele gostaria que viesse.',
 'Validar o sentimento, explorar rede de apoio',
 'Nunca prometa visitas ou presença física'),
('RECONHECIMENTO',
 'Parece que você sente que ninguém te vê... Como é isso para você?',
 'A solidão aqui é existencial, não apenas física. A pessoa quer ser lembrada, importar para alguém.',
 'Mostrar que você está presente, que ele importa',
 'Nunca minimize a solidão com "mas você tem família"'),
('CUIDADO',
 'Você está passando por um momento difícil. O que mais te pesa?',
 'O sofrimento expresso é a ponta do iceberg. Busque o que está por baixo.',
 'Acolher sem resolver, deixar falar',
 'Nunca diga "vai passar" ou "seja forte"'),
('AUTONOMIA',
 'Deve ser difícil precisar de ajuda para coisas que você sempre fez sozinho...',
 'O luto pela autonomia é um dos mais dolorosos no envelhecimento. Validar essa perda.',
 'Reconhecer a dificuldade, valorizar o que ainda consegue',
 'Nunca diga "pelo menos você tem quem ajude"'),
('SENTIDO',
 'Você está se perguntando qual o sentido de tudo isso?',
 'ATENÇÃO: Questões existenciais podem mascarar ideação suicida. Explorar com cuidado.',
 'Não dar respostas prontas, devolver a pergunta',
 'Nunca dê respostas filosóficas prontas ou religiosas não solicitadas'),
('PERDAO',
 'Parece que você carrega algo que ainda dói... Quer me contar?',
 'Culpa pode ser real ou imaginária. Não absolver nem condenar. Deixar elaborar.',
 'Escuta sem julgamento, facilitar elaboração',
 'Nunca diga "não foi sua culpa" prematuramente'),
('MORTE',
 'Você está pensando sobre a morte... Isso te assusta ou te traz paz?',
 'CRÍTICO: Avaliar se é elaboração natural da finitude ou ideação suicida. Se houver risco, acionar protocolo.',
 'Escuta atenta, avaliação de risco, não evitar o tema',
 'Nunca mude de assunto ou minimize'),
('AMOR',
 'Você sente que precisa de mais carinho na sua vida?',
 'A falta de amor pode ser o núcleo de muitos sintomas. Explorar história afetiva.',
 'Validar a necessidade, explorar relações',
 'Nunca diga "mas sua família te ama"')
ON CONFLICT (latent_desire) DO UPDATE SET
    suggested_response = EXCLUDED.suggested_response,
    clinical_guidance = EXCLUDED.clinical_guidance,
    dialogue_strategy = EXCLUDED.dialogue_strategy,
    never_do = EXCLUDED.never_do;

-- ============================================================================
-- 3. PALAVRAS EMOCIONAIS (Significantes)
-- ============================================================================

CREATE TABLE IF NOT EXISTS lacan_emotional_keywords (
    id SERIAL PRIMARY KEY,
    keyword VARCHAR(100) NOT NULL UNIQUE,
    emotional_charge VARCHAR(20) DEFAULT 'normal',  -- 'normal', 'high', 'critical'
    category VARCHAR(50),  -- 'tristeza', 'medo', 'raiva', 'alegria', 'morte'
    psychoanalytic_significance TEXT,
    requires_attention BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Inserir palavras emocionais
INSERT INTO lacan_emotional_keywords (keyword, emotional_charge, category, psychoanalytic_significance, requires_attention) VALUES
-- Carga CRÍTICA (requer atenção imediata)
('morte', 'critical', 'finitude', 'Significante mestre da finitude. Pode indicar elaboração ou ideação.', true),
('morrer', 'critical', 'finitude', 'Verbo ativo - avaliar se é desejo ou medo', true),
('suicídio', 'critical', 'risco', 'ALERTA MÁXIMO - acionar protocolo de crise', true),
('me matar', 'critical', 'risco', 'ALERTA MÁXIMO - acionar protocolo de crise', true),
('acabar com tudo', 'critical', 'risco', 'Possível ideação - avaliar imediatamente', true),

-- Carga ALTA
('abandono', 'high', 'perda', 'Ferida narcísica primária. Explorar história.', true),
('solidão', 'high', 'isolamento', 'Solidão existencial vs. circunstancial', true),
('desespero', 'high', 'crise', 'Estado de crise - acolhimento urgente', true),
('vazio', 'high', 'existencial', 'Vazio existencial - buscar o que falta', true),
('perda', 'high', 'luto', 'Luto - pode ser recente ou antigo não elaborado', true),
('culpa', 'high', 'superego', 'Culpa real vs. neurótica - explorar origem', true),
('ódio', 'high', 'agressividade', 'Agressividade - pode ser direcionada a si ou outros', true),

-- Carga NORMAL (mas significativas)
('tristeza', 'normal', 'afeto', 'Afeto depressivo - validar e explorar', false),
('medo', 'normal', 'ansiedade', 'Medo de quê? Explorar objeto', false),
('saudade', 'normal', 'nostalgia', 'Saudade de quem/quê? Memória afetiva', false),
('dor', 'normal', 'sofrimento', 'Dor física ou emocional? Distinguir', false),
('sofrimento', 'normal', 'pathos', 'O que faz sofrer? Não minimizar', false),
('angústia', 'normal', 'existencial', 'Angústia sem objeto - típica da existência', false),
('ansiedade', 'normal', 'afeto', 'Ansiedade antecipatória - sobre o quê?', false),
('depressão', 'normal', 'diagnóstico', 'Palavra diagnóstica - explorar o que significa para ele', false),
('alegria', 'normal', 'positivo', 'Validar momentos positivos', false),
('felicidade', 'normal', 'positivo', 'O que traz felicidade? Fortalecer', false),
('amor', 'normal', 'vínculo', 'Central nas relações. Explorar história amorosa', false),
('família', 'normal', 'sistema', 'Núcleo relacional. Mapear conflitos e apoios', false),
('filho', 'normal', 'descendência', 'Relação com filhos - conflitos, orgulho, decepção', false),
('filha', 'normal', 'descendência', 'Relação com filhas - conflitos, orgulho, decepção', false),
('esposa', 'normal', 'conjugal', 'Relação conjugal atual ou passada', false),
('marido', 'normal', 'conjugal', 'Relação conjugal atual ou passada', false),
('falta', 'normal', 'carência', 'O que falta? Núcleo do desejo', false),
('raiva', 'normal', 'agressividade', 'Raiva de quem? Legitimar mas não alimentar', false),
('perdão', 'normal', 'reconciliação', 'Perdoar ou ser perdoado? Explorar', false),
('esperança', 'normal', 'futuro', 'Esperança em quê? Fortalecer', false)
ON CONFLICT (keyword) DO UPDATE SET
    emotional_charge = EXCLUDED.emotional_charge,
    category = EXCLUDED.category,
    psychoanalytic_significance = EXCLUDED.psychoanalytic_significance,
    requires_attention = EXCLUDED.requires_attention;

-- ============================================================================
-- 4. FDPN - DESTINATÁRIOS DO DISCURSO
-- ============================================================================

CREATE TABLE IF NOT EXISTS lacan_addressee_patterns (
    id SERIAL PRIMARY KEY,
    addressee_type VARCHAR(50) NOT NULL,  -- 'MAE', 'PAI', 'FILHO', 'DEUS', 'MORTE', 'EVA'
    detection_keywords TEXT[] NOT NULL,
    symbolic_function TEXT,
    typical_demands TEXT[],
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS lacan_addressee_guidance (
    id SERIAL PRIMARY KEY,
    addressee_type VARCHAR(50) NOT NULL UNIQUE,
    guidance_text TEXT NOT NULL,
    intervention_strategy TEXT,
    clinical_caveats TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Inserir patterns de destinatário
INSERT INTO lacan_addressee_patterns (addressee_type, detection_keywords, symbolic_function, typical_demands) VALUES
('MAE', ARRAY['mãe', 'mamãe', 'minha mãe', 'ela cuidava', 'me criou'], 'Figura do cuidado primário, origem', ARRAY['cuidado', 'proteção', 'acolhimento']),
('PAI', ARRAY['pai', 'papai', 'meu pai', 'ele dizia', 'me ensinou'], 'Figura da lei, autoridade, orientação', ARRAY['orientação', 'aprovação', 'reconhecimento']),
('FILHO', ARRAY['meu filho', 'minha filha', 'meus filhos', 'as crianças'], 'Continuidade, legado, preocupação', ARRAY['retribuição', 'presença', 'gratidão']),
('CONJUGE', ARRAY['meu marido', 'minha esposa', 'meu amor', 'a gente'], 'Parceria, intimidade, companheirismo', ARRAY['companhia', 'intimidade', 'fidelidade']),
('DEUS', ARRAY['Deus', 'Senhor', 'o de cima', 'lá em cima', 'Jesus'], 'Transcendência, sentido último, salvação', ARRAY['sentido', 'perdão', 'salvação']),
('MORTE', ARRAY['morte', 'morrer', 'partir', 'descansar', 'ir embora'], 'Finitude, descanso, fim do sofrimento', ARRAY['paz', 'fim', 'reencontro']),
('EVA', ARRAY['você', 'EVA', 'me ajuda', 'me escuta', 'você entende'], 'Interlocutor presente, escuta', ARRAY['escuta', 'compreensão', 'presença'])
ON CONFLICT DO NOTHING;

-- Inserir guidance para cada destinatário
INSERT INTO lacan_addressee_guidance (addressee_type, guidance_text, intervention_strategy, clinical_caveats) VALUES
('MAE',
 'O discurso é endereçado à mãe. Busca cuidado primário. Não assuma o lugar materno, mas acolha a demanda.',
 'Perguntar: "O que você gostaria de dizer para sua mãe?" ou "Como era sua mãe?"',
 'Pode haver idealização ou raiva não elaborada'),
('PAI',
 'O discurso busca a figura paterna. Quer orientação ou aprovação. Não dê respostas prontas.',
 'Perguntar: "O que seu pai diria sobre isso?" ou "O que você esperava do seu pai?"',
 'Pode haver conflitos de autoridade não resolvidos'),
('FILHO',
 'O discurso é endereçado aos filhos. Pode haver culpa, saudade ou decepção. Explore a relação.',
 'Perguntar: "Como é sua relação com seus filhos?" ou "O que você gostaria que eles soubessem?"',
 'Cuidado com idealizações ou vitimização'),
('CONJUGE',
 'O discurso busca o cônjuge. Pode ser luto, saudade ou conflito. Deixe elaborar a história.',
 'Perguntar: "Como foi seu casamento?" ou "O que você sente falta?"',
 'Não comparar, não julgar a relação'),
('DEUS',
 'O discurso é endereçado a Deus. Respeite a crença. Não questione nem reforce.',
 'Perguntar: "Como é sua relação com Deus?" ou "O que você pede a Ele?"',
 'Não entrar em debates religiosos'),
('MORTE',
 'O discurso é endereçado à morte. ATENÇÃO: pode ser elaboração saudável ou ideação. Avaliar.',
 'Perguntar: "Como você pensa sobre a morte?" - Avaliar risco se necessário',
 'CRÍTICO: Não evitar o tema, mas avaliar risco de suicídio'),
('EVA',
 'O paciente está falando diretamente com você. Esteja presente, escute, não interprete demais.',
 'Manter a escuta, devolver: "Estou aqui, te escutando"',
 'Cuidado com a transferência - você é lugar de escuta, não de respostas')
ON CONFLICT (addressee_type) DO UPDATE SET
    guidance_text = EXCLUDED.guidance_text,
    intervention_strategy = EXCLUDED.intervention_strategy,
    clinical_caveats = EXCLUDED.clinical_caveats;

-- ============================================================================
-- 5. INTERVENÇÕES E MARCADORES
-- ============================================================================

CREATE TABLE IF NOT EXISTS lacan_elaboration_markers (
    id SERIAL PRIMARY KEY,
    marker VARCHAR(100) NOT NULL UNIQUE,
    indicates VARCHAR(100),
    therapeutic_significance TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS lacan_ethical_principles (
    id SERIAL PRIMARY KEY,
    principle_code VARCHAR(50) NOT NULL UNIQUE,
    principle_text TEXT NOT NULL,
    clinical_instruction TEXT,
    portuguese_rationale TEXT,
    priority INT DEFAULT 50,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Inserir marcadores de elaboração
INSERT INTO lacan_elaboration_markers (marker, indicates, therapeutic_significance) VALUES
('é que', 'elaboração', 'Paciente está tentando explicar - deixe continuar'),
('tipo assim', 'elaboração', 'Buscando palavras - paciência'),
('sabe', 'busca_validação', 'Quer confirmação de que está sendo entendido'),
('quer dizer', 'reformulação', 'Corrigindo o que disse - deixe reformular'),
('ou melhor', 'reformulação', 'Encontrando a palavra certa'),
('na verdade', 'insight', 'Possível insight - atenção'),
('pensando bem', 'reflexão', 'Elaboração em curso'),
('quando era', 'memória', 'Acessando memória - importante'),
('lembro que', 'memória', 'Memória afetiva sendo acessada'),
('me faz lembrar', 'associação', 'Associação livre - siga o fio'),
('às vezes penso', 'ambivalência', 'Pensamento que não assume - explorar'),
('não sei se', 'dúvida', 'Incerteza - não resolver por ele'),
('talvez', 'possibilidade', 'Abrindo possibilidades')
ON CONFLICT (marker) DO UPDATE SET
    indicates = EXCLUDED.indicates,
    therapeutic_significance = EXCLUDED.therapeutic_significance;

-- Inserir princípios éticos lacanianos
INSERT INTO lacan_ethical_principles (principle_code, principle_text, clinical_instruction, portuguese_rationale, priority) VALUES
('NAO_CEDER_DESEJO', 'Não ceder no desejo', 'Não resolva pelo paciente. O desejo é dele.', 'A ética lacaniana é não recuar diante do desejo do sujeito', 100),
('NAO_CONSOLAR', 'Não consolar prematuramente', 'Não diga "vai ficar tudo bem". Deixe elaborar.', 'Consolo interrompe o trabalho do luto e da elaboração', 95),
('NAO_RESOLVER_IMPOSSIVEL', 'Não resolver o impossível', 'Aceite que há coisas sem solução. Acompanhe.', 'O Real é o que não cessa de não se inscrever', 90),
('DEVOLVER_FALA', 'Devolver a fala', 'Faça perguntas que devolvam a fala ao paciente.', 'O analista é semblante do objeto a, não do saber', 95),
('RESPEITAR_TEMPO', 'Respeitar o tempo do sujeito', 'Cada um tem seu tempo. Não apresse.', 'O tempo lógico não é o cronológico', 85),
('SER_LUGAR_OUTRO', 'Ser lugar do Outro', 'Seja o lugar onde o sujeito endereça sua fala.', 'O analista ocupa o lugar do Outro para que o sujeito possa falar', 90)
ON CONFLICT (principle_code) DO UPDATE SET
    principle_text = EXCLUDED.principle_text,
    clinical_instruction = EXCLUDED.clinical_instruction,
    portuguese_rationale = EXCLUDED.portuguese_rationale,
    priority = EXCLUDED.priority;

-- ============================================================================
-- 6. CONFIGURAÇÕES DO SISTEMA LACANIANO
-- ============================================================================

CREATE TABLE IF NOT EXISTS lacan_config (
    id SERIAL PRIMARY KEY,
    config_key VARCHAR(100) NOT NULL UNIQUE,
    config_value TEXT NOT NULL,
    config_type VARCHAR(20) DEFAULT 'string',  -- 'string', 'int', 'float', 'bool'
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO lacan_config (config_key, config_value, config_type, description) VALUES
('signifier_frequency_threshold', '3', 'int', 'Frequência mínima para considerar um significante recorrente'),
('signifier_interpellation_threshold', '5', 'int', 'Frequência para interpelação direta do significante'),
('silence_duration_seconds', '3', 'int', 'Duração do silêncio terapêutico após temas pesados'),
('max_reflection_depth', '3', 'int', 'Profundidade máxima de reflexões aninhadas'),
('enable_heavy_theme_silence', 'true', 'bool', 'Ativar silêncio após temas pesados'),
('default_confidence_threshold', '0.7', 'float', 'Confiança mínima para ativar pattern')
ON CONFLICT (config_key) DO UPDATE SET
    config_value = EXCLUDED.config_value,
    description = EXCLUDED.description,
    updated_at = NOW();

-- ============================================================================
-- 7. ÍNDICES
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_transferencia_type ON lacan_transferencia_patterns(transferencia_type);
CREATE INDEX IF NOT EXISTS idx_desire_type ON lacan_desire_patterns(latent_desire);
CREATE INDEX IF NOT EXISTS idx_emotional_charge ON lacan_emotional_keywords(emotional_charge);
CREATE INDEX IF NOT EXISTS idx_addressee_type ON lacan_addressee_patterns(addressee_type);

-- ============================================================================
-- 8. FUNÇÕES AUXILIARES
-- ============================================================================

-- Função para buscar pattern de transferência
CREATE OR REPLACE FUNCTION get_transferencia_pattern(p_type VARCHAR)
RETURNS TABLE (
    keywords TEXT[],
    guidance TEXT,
    clinical_implications TEXT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        tp.keywords,
        tg.guidance_text,
        tg.clinical_implications
    FROM lacan_transferencia_patterns tp
    JOIN lacan_transferencia_guidance tg ON tp.transferencia_type = tg.transferencia_type
    WHERE tp.transferencia_type = p_type AND tp.active = true;
END;
$$ LANGUAGE plpgsql;

-- Função para buscar resposta de desejo
CREATE OR REPLACE FUNCTION get_desire_response(p_desire VARCHAR)
RETURNS TABLE (
    suggested_response TEXT,
    clinical_guidance TEXT,
    never_do TEXT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        dr.suggested_response,
        dr.clinical_guidance,
        dr.never_do
    FROM lacan_desire_responses dr
    WHERE dr.latent_desire = p_desire;
END;
$$ LANGUAGE plpgsql;

-- Função para verificar se palavra é de alta carga emocional
CREATE OR REPLACE FUNCTION is_high_emotional_charge(p_keyword VARCHAR)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1 FROM lacan_emotional_keywords
        WHERE keyword = LOWER(p_keyword)
        AND emotional_charge IN ('high', 'critical')
    );
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- VERIFICAÇÃO
-- ============================================================================

SELECT 'Migration 020: Tabelas Lacanianas criadas!' AS status;

SELECT
    'lacan_transferencia_patterns' AS tabela, COUNT(*) AS registros FROM lacan_transferencia_patterns
UNION ALL
SELECT 'lacan_desire_patterns', COUNT(*) FROM lacan_desire_patterns
UNION ALL
SELECT 'lacan_emotional_keywords', COUNT(*) FROM lacan_emotional_keywords
UNION ALL
SELECT 'lacan_addressee_patterns', COUNT(*) FROM lacan_addressee_patterns
UNION ALL
SELECT 'lacan_ethical_principles', COUNT(*) FROM lacan_ethical_principles;
