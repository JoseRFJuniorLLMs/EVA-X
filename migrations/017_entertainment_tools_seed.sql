-- =====================================================
-- MIGRATION 017: ENTERTAINMENT TOOLS - 30 Ferramentas
-- =====================================================
-- Ferramentas de entretenimento e bem-estar para idosos
-- Categorias: music, games, stories, wellness, social, media
-- =====================================================

-- =====================================================
-- CATEGORIA 1: MÚSICA E ÁUDIO (6 ferramentas)
-- =====================================================

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES

-- 1. play_nostalgic_music
(
    'play_nostalgic_music',
    'Tocar Música Nostálgica',
    'Toca músicas da época do paciente (anos 50-80) baseado em preferências salvas. Pode filtrar por década, artista, gênero ou humor. Ideal para terapia de reminiscência e momentos de relaxamento.',
    'entertainment',
    'music',
    '{
        "type": "OBJECT",
        "properties": {
            "decade": {
                "type": "STRING",
                "description": "Década da música (1950s, 1960s, 1970s, 1980s)",
                "enum": ["1950s", "1960s", "1970s", "1980s", "any"]
            },
            "artist": {
                "type": "STRING",
                "description": "Nome do artista ou banda (ex: Roberto Carlos, Elis Regina)"
            },
            "genre": {
                "type": "STRING",
                "description": "Gênero musical",
                "enum": ["mpb", "samba", "bossa_nova", "sertanejo", "forro", "bolero", "internacional", "any"]
            },
            "mood": {
                "type": "STRING",
                "description": "Humor desejado para a música",
                "enum": ["alegre", "calma", "romantica", "animada", "nostalgica", "any"]
            }
        },
        "required": []
    }'::jsonb,
    true,
    'internal',
    70,
    false,
    'Use quando o paciente pedir música, quiser relaxar, estiver triste, ou durante terapia de reminiscência.',
    '["coloca uma música", "quero ouvir Roberto Carlos", "toca algo dos anos 60", "música pra relaxar", "bota um sambinha"]'::jsonb,
    '["música", "entretenimento", "relaxamento", "nostalgia"]'::jsonb
),

-- 2. play_radio_station
(
    'play_radio_station',
    'Sintonizar Rádio',
    'Sintoniza estações de rádio online. Inclui rádios de notícias, música, religiosas e locais.',
    'entertainment',
    'music',
    '{
        "type": "OBJECT",
        "properties": {
            "station_type": {
                "type": "STRING",
                "description": "Tipo de estação",
                "enum": ["news", "music", "religious", "local", "sports"]
            },
            "station_name": {
                "type": "STRING",
                "description": "Nome específico da estação (ex: CBN, Jovem Pan, Canção Nova)"
            }
        },
        "required": ["station_type"]
    }'::jsonb,
    true,
    'internal',
    65,
    false,
    'Use quando o paciente quiser ouvir rádio, notícias, ou programas específicos.',
    '["liga a rádio", "quero ouvir notícias", "coloca a CBN", "rádio gospel", "rádio de esportes"]'::jsonb,
    '["rádio", "notícias", "entretenimento"]'::jsonb
),

-- 3. nature_sounds
(
    'nature_sounds',
    'Sons da Natureza',
    'Reproduz sons relaxantes da natureza: chuva, mar, floresta, pássaros, fogueira. Ideal para relaxamento, meditação e ajudar a dormir.',
    'entertainment',
    'music',
    '{
        "type": "OBJECT",
        "properties": {
            "sound_type": {
                "type": "STRING",
                "description": "Tipo de som da natureza",
                "enum": ["rain", "ocean", "forest", "birds", "fireplace", "river", "thunderstorm", "wind"]
            },
            "duration_minutes": {
                "type": "INTEGER",
                "description": "Duração em minutos (padrão: 30)"
            },
            "volume": {
                "type": "STRING",
                "description": "Volume do som",
                "enum": ["low", "medium", "high"]
            }
        },
        "required": ["sound_type"]
    }'::jsonb,
    true,
    'internal',
    60,
    false,
    'Use para relaxamento, ajudar a dormir, meditação, ou quando paciente estiver ansioso.',
    '["som de chuva", "barulho do mar", "quero relaxar", "sons pra dormir", "som de passarinhos"]'::jsonb,
    '["relaxamento", "sono", "natureza", "meditação"]'::jsonb
),

-- 4. audiobook_reader
(
    'audiobook_reader',
    'Ler Audiobook',
    'Lê audiobooks e livros em voz alta. Pode pausar, retomar, ajustar velocidade. Biblioteca inclui clássicos brasileiros e best-sellers.',
    'entertainment',
    'media',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação a executar",
                "enum": ["play", "pause", "resume", "stop", "list", "search"]
            },
            "book_title": {
                "type": "STRING",
                "description": "Título do livro"
            },
            "chapter": {
                "type": "INTEGER",
                "description": "Número do capítulo"
            },
            "speed": {
                "type": "STRING",
                "description": "Velocidade de leitura",
                "enum": ["slow", "normal", "fast"]
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    55,
    false,
    'Use quando paciente quiser ouvir histórias, livros, ou não conseguir ler sozinho.',
    '["lê um livro pra mim", "continua o livro", "quero ouvir uma história", "para a leitura"]'::jsonb,
    '["livros", "leitura", "histórias", "entretenimento"]'::jsonb
),

-- 5. podcast_player
(
    'podcast_player',
    'Tocar Podcast',
    'Reproduz podcasts populares. Categorias incluem saúde, história, humor, espiritualidade, notícias.',
    'entertainment',
    'media',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação a executar",
                "enum": ["play", "pause", "resume", "list", "search"]
            },
            "category": {
                "type": "STRING",
                "description": "Categoria do podcast",
                "enum": ["health", "history", "humor", "spirituality", "news", "culture"]
            },
            "podcast_name": {
                "type": "STRING",
                "description": "Nome específico do podcast"
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    50,
    false,
    'Use quando paciente quiser ouvir podcasts ou programas sobre temas específicos.',
    '["coloca um podcast", "quero ouvir algo sobre história", "podcast de saúde"]'::jsonb,
    '["podcast", "entretenimento", "educação"]'::jsonb
),

-- 6. religious_content
(
    'religious_content',
    'Conteúdo Religioso',
    'Reproduz orações, terços, reflexões bíblicas, hinos e músicas religiosas. Suporta católico, evangélico, espírita.',
    'entertainment',
    'spiritual',
    '{
        "type": "OBJECT",
        "properties": {
            "content_type": {
                "type": "STRING",
                "description": "Tipo de conteúdo",
                "enum": ["prayer", "rosary", "bible_reflection", "hymn", "mass", "meditation"]
            },
            "religion": {
                "type": "STRING",
                "description": "Tradição religiosa",
                "enum": ["catholic", "evangelical", "spiritist", "generic"]
            },
            "specific_prayer": {
                "type": "STRING",
                "description": "Oração específica (ex: Pai Nosso, Ave Maria, Salmo 23)"
            }
        },
        "required": ["content_type"]
    }'::jsonb,
    true,
    'internal',
    65,
    false,
    'Use quando paciente quiser rezar, ouvir orações, ou buscar conforto espiritual.',
    '["reza comigo", "quero ouvir o terço", "lê um salmo", "música gospel", "reflexão do dia"]'::jsonb,
    '["religião", "espiritualidade", "oração", "conforto"]'::jsonb
);

-- =====================================================
-- CATEGORIA 2: JOGOS E EXERCÍCIOS MENTAIS (6 ferramentas)
-- =====================================================

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES

-- 7. play_trivia_game
(
    'play_trivia_game',
    'Quiz de Conhecimentos',
    'Jogo de perguntas e respostas sobre diversos temas. Adapta dificuldade ao paciente. Temas: história do Brasil, cultura popular, música, geografia.',
    'entertainment',
    'games',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação do jogo",
                "enum": ["start", "answer", "hint", "skip", "score", "end"]
            },
            "theme": {
                "type": "STRING",
                "description": "Tema das perguntas",
                "enum": ["brazil_history", "music", "geography", "culture", "sports", "random"]
            },
            "difficulty": {
                "type": "STRING",
                "description": "Nível de dificuldade",
                "enum": ["easy", "medium", "hard"]
            },
            "answer": {
                "type": "STRING",
                "description": "Resposta do paciente"
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    60,
    false,
    'Use quando paciente quiser jogar, se distrair, ou exercitar a memória.',
    '["vamos jogar?", "faz uma pergunta", "quiz de história", "jogo de perguntas"]'::jsonb,
    '["jogos", "quiz", "memória", "entretenimento"]'::jsonb
),

-- 8. memory_game
(
    'memory_game',
    'Jogo da Memória',
    'Exercício de memória por voz. EVA diz uma sequência e paciente repete. Aumenta gradualmente a dificuldade. Ótimo para treino cognitivo.',
    'entertainment',
    'games',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação do jogo",
                "enum": ["start", "repeat", "check", "score", "end"]
            },
            "game_type": {
                "type": "STRING",
                "description": "Tipo de memória",
                "enum": ["numbers", "words", "colors", "objects"]
            },
            "patient_answer": {
                "type": "STRING",
                "description": "Resposta do paciente"
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    65,
    false,
    'Use para treino cognitivo, exercitar memória, ou quando paciente quiser jogos mentais.',
    '["jogo da memória", "exercício de memória", "treinar a cabeça", "repete os números"]'::jsonb,
    '["memória", "cognitivo", "treino", "jogos"]'::jsonb
),

-- 9. word_association
(
    'word_association',
    'Associação de Palavras',
    'Jogo de associação de palavras. EVA diz uma palavra e paciente responde com palavra relacionada. Estimula conexões neurais.',
    'entertainment',
    'games',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação do jogo",
                "enum": ["start", "respond", "end"]
            },
            "category": {
                "type": "STRING",
                "description": "Categoria das palavras",
                "enum": ["general", "food", "places", "people", "objects"]
            },
            "response": {
                "type": "STRING",
                "description": "Resposta do paciente"
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    55,
    false,
    'Use para estimulação cognitiva, quando paciente estiver entediado, ou como aquecimento mental.',
    '["vamos associar palavras", "jogo de palavras", "fala uma palavra"]'::jsonb,
    '["palavras", "cognitivo", "associação", "jogos"]'::jsonb
),

-- 10. brain_training
(
    'brain_training',
    'Treino Cerebral',
    'Exercícios cognitivos variados: cálculos simples, completar sequências, encontrar diferenças. Adaptativo ao nível do paciente.',
    'entertainment',
    'games',
    '{
        "type": "OBJECT",
        "properties": {
            "exercise_type": {
                "type": "STRING",
                "description": "Tipo de exercício",
                "enum": ["math", "sequences", "categories", "opposites", "analogies"]
            },
            "difficulty": {
                "type": "STRING",
                "description": "Dificuldade",
                "enum": ["very_easy", "easy", "medium"]
            },
            "action": {
                "type": "STRING",
                "description": "Ação",
                "enum": ["start", "answer", "hint", "next", "end"]
            },
            "answer": {
                "type": "STRING",
                "description": "Resposta do paciente"
            }
        },
        "required": ["exercise_type", "action"]
    }'::jsonb,
    true,
    'internal',
    65,
    false,
    'Use para manter mente ativa, prevenção cognitiva, ou quando paciente quiser "exercitar a cabeça".',
    '["exercício mental", "treino cerebral", "conta de cabeça", "exercitar a mente"]'::jsonb,
    '["cognitivo", "treino", "mente", "exercícios"]'::jsonb
),

-- 11. complete_the_lyrics
(
    'complete_the_lyrics',
    'Complete a Letra',
    'Jogo musical onde EVA canta parte de uma música famosa e paciente completa. Usa músicas da época do paciente.',
    'entertainment',
    'games',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação do jogo",
                "enum": ["start", "answer", "skip", "hint", "score"]
            },
            "decade": {
                "type": "STRING",
                "description": "Década das músicas",
                "enum": ["1950s", "1960s", "1970s", "1980s", "mixed"]
            },
            "answer": {
                "type": "STRING",
                "description": "Continuação da letra pelo paciente"
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    60,
    false,
    'Use para diversão, exercício de memória musical, ou durante momentos de nostalgia.',
    '["complete a música", "jogo de música", "continua a letra", "adivinha a música"]'::jsonb,
    '["música", "jogos", "memória", "nostalgia"]'::jsonb
),

-- 12. riddles_and_jokes
(
    'riddles_and_jokes',
    'Charadas e Piadas',
    'Conta charadas, adivinhas e piadas adequadas para idosos. Humor leve e respeitoso.',
    'entertainment',
    'games',
    '{
        "type": "OBJECT",
        "properties": {
            "content_type": {
                "type": "STRING",
                "description": "Tipo de conteúdo",
                "enum": ["joke", "riddle", "tongue_twister", "funny_story"]
            },
            "theme": {
                "type": "STRING",
                "description": "Tema",
                "enum": ["general", "animals", "family", "daily_life", "classic"]
            }
        },
        "required": ["content_type"]
    }'::jsonb,
    true,
    'internal',
    55,
    false,
    'Use para descontrair, quando paciente estiver triste, ou pedir algo engraçado.',
    '["conta uma piada", "faz uma adivinha", "trava-língua", "me faz rir"]'::jsonb,
    '["humor", "piadas", "diversão", "entretenimento"]'::jsonb
);

-- =====================================================
-- CATEGORIA 3: HISTÓRIAS E NARRATIVAS (5 ferramentas)
-- =====================================================

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES

-- 13. story_generator
(
    'story_generator',
    'Gerador de Histórias',
    'Gera histórias personalizadas baseadas nas preferências e memórias do paciente. Pode incluir nomes de familiares e lugares conhecidos.',
    'entertainment',
    'stories',
    '{
        "type": "OBJECT",
        "properties": {
            "story_type": {
                "type": "STRING",
                "description": "Tipo de história",
                "enum": ["adventure", "romance", "family", "childhood", "fantasy", "historical"]
            },
            "include_family": {
                "type": "BOOLEAN",
                "description": "Incluir nomes de familiares na história"
            },
            "length": {
                "type": "STRING",
                "description": "Tamanho da história",
                "enum": ["short", "medium", "long"]
            },
            "setting": {
                "type": "STRING",
                "description": "Cenário (ex: fazenda, cidade, praia)"
            }
        },
        "required": ["story_type"]
    }'::jsonb,
    true,
    'internal',
    55,
    false,
    'Use quando paciente quiser ouvir histórias, estiver entediado, ou antes de dormir.',
    '["conta uma história", "inventa uma história", "história de aventura", "história pra dormir"]'::jsonb,
    '["histórias", "narrativa", "entretenimento", "imaginação"]'::jsonb
),

-- 14. reminiscence_therapy
(
    'reminiscence_therapy',
    'Terapia de Reminiscência',
    'Guia conversa terapêutica sobre memórias do passado. Usa fotos, músicas e perguntas para evocar lembranças positivas. Técnica validada para bem-estar de idosos.',
    'entertainment',
    'therapy',
    '{
        "type": "OBJECT",
        "properties": {
            "theme": {
                "type": "STRING",
                "description": "Tema da reminiscência",
                "enum": ["childhood", "youth", "marriage", "career", "travels", "holidays", "family", "friends"]
            },
            "use_music": {
                "type": "BOOLEAN",
                "description": "Usar músicas da época como gatilho"
            },
            "use_photos": {
                "type": "BOOLEAN",
                "description": "Usar fotos do paciente (se disponíveis)"
            }
        },
        "required": ["theme"]
    }'::jsonb,
    true,
    'internal',
    70,
    false,
    'Use para conectar com o passado, melhorar humor, ou quando paciente mencionar saudades.',
    '["lembra da minha infância", "vamos falar do passado", "conta da minha juventude", "saudade de antigamente"]'::jsonb,
    '["memória", "terapia", "reminiscência", "bem-estar"]'::jsonb
),

-- 15. biography_writer
(
    'biography_writer',
    'Escritor de Biografia',
    'Ajuda a construir a biografia do paciente através de conversas. Salva histórias de vida para deixar como legado para família.',
    'entertainment',
    'stories',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação",
                "enum": ["start_session", "continue", "read_back", "export", "add_photo"]
            },
            "life_chapter": {
                "type": "STRING",
                "description": "Capítulo da vida",
                "enum": ["birth_childhood", "youth", "love_marriage", "career", "parenthood", "wisdom", "legacy"]
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    60,
    false,
    'Use quando paciente quiser contar sua história, deixar legado, ou durante sessões de memória.',
    '["quero contar minha história", "escreve minha biografia", "história da minha vida", "deixar para meus netos"]'::jsonb,
    '["biografia", "legado", "memória", "família"]'::jsonb
),

-- 16. read_newspaper
(
    'read_newspaper',
    'Ler Notícias',
    'Lê manchetes e notícias do dia em voz alta. Filtra por categoria. Evita notícias muito negativas ou violentas.',
    'entertainment',
    'media',
    '{
        "type": "OBJECT",
        "properties": {
            "category": {
                "type": "STRING",
                "description": "Categoria de notícias",
                "enum": ["general", "sports", "entertainment", "health", "local", "positive"]
            },
            "source": {
                "type": "STRING",
                "description": "Fonte preferida",
                "enum": ["g1", "uol", "folha", "estadao", "local"]
            },
            "detail_level": {
                "type": "STRING",
                "description": "Nível de detalhe",
                "enum": ["headlines", "summary", "full"]
            }
        },
        "required": ["category"]
    }'::jsonb,
    true,
    'internal',
    55,
    false,
    'Use quando paciente quiser saber das notícias, se manter informado, ou de manhã.',
    '["lê as notícias", "o que aconteceu hoje?", "notícias de esporte", "manchetes do dia"]'::jsonb,
    '["notícias", "informação", "atualidades"]'::jsonb
),

-- 17. daily_horoscope
(
    'daily_horoscope',
    'Horóscopo do Dia',
    'Lê o horóscopo diário do signo do paciente. Mensagens sempre positivas e motivacionais.',
    'entertainment',
    'daily',
    '{
        "type": "OBJECT",
        "properties": {
            "sign": {
                "type": "STRING",
                "description": "Signo do zodíaco",
                "enum": ["aries", "taurus", "gemini", "cancer", "leo", "virgo", "libra", "scorpio", "sagittarius", "capricorn", "aquarius", "pisces"]
            }
        },
        "required": []
    }'::jsonb,
    true,
    'internal',
    40,
    false,
    'Use de manhã, quando paciente perguntar do signo, ou quiser uma mensagem do dia.',
    '["meu horóscopo", "o que diz meu signo?", "previsão do dia", "sou de leão"]'::jsonb,
    '["horóscopo", "diário", "motivação"]'::jsonb
);

-- =====================================================
-- CATEGORIA 4: BEM-ESTAR E SAÚDE (6 ferramentas)
-- =====================================================

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES

-- 18. guided_meditation
(
    'guided_meditation',
    'Meditação Guiada',
    'Conduz meditação guiada por voz. Diferentes técnicas: mindfulness, body scan, visualização, gratidão.',
    'wellness',
    'meditation',
    '{
        "type": "OBJECT",
        "properties": {
            "technique": {
                "type": "STRING",
                "description": "Técnica de meditação",
                "enum": ["mindfulness", "body_scan", "visualization", "gratitude", "loving_kindness", "sleep"]
            },
            "duration_minutes": {
                "type": "INTEGER",
                "description": "Duração em minutos (5, 10, 15, 20)"
            },
            "background_sound": {
                "type": "STRING",
                "description": "Som de fundo",
                "enum": ["none", "nature", "music", "bells"]
            }
        },
        "required": ["technique"]
    }'::jsonb,
    true,
    'internal',
    70,
    false,
    'Use para ansiedade, antes de dormir, ou quando paciente quiser relaxar profundamente.',
    '["medita comigo", "quero meditar", "meditação pra dormir", "relaxamento profundo"]'::jsonb,
    '["meditação", "relaxamento", "bem-estar", "ansiedade"]'::jsonb
),

-- 19. breathing_exercises
(
    'breathing_exercises',
    'Exercícios de Respiração',
    'Guia exercícios de respiração: 4-7-8, respiração diafragmática, box breathing. Ótimo para ansiedade e pânico.',
    'wellness',
    'breathing',
    '{
        "type": "OBJECT",
        "properties": {
            "technique": {
                "type": "STRING",
                "description": "Técnica de respiração",
                "enum": ["4-7-8", "box_breathing", "diaphragmatic", "calming", "energizing"]
            },
            "cycles": {
                "type": "INTEGER",
                "description": "Número de ciclos (padrão: 5)"
            }
        },
        "required": ["technique"]
    }'::jsonb,
    true,
    'internal',
    75,
    false,
    'Use para ansiedade, pânico, insônia, ou como rotina de relaxamento.',
    '["respira comigo", "exercício de respiração", "tô ansiosa", "me ajuda a acalmar"]'::jsonb,
    '["respiração", "ansiedade", "relaxamento", "bem-estar"]'::jsonb
),

-- 20. chair_exercises
(
    'chair_exercises',
    'Exercícios na Cadeira',
    'Guia exercícios físicos leves que podem ser feitos sentado. Alongamentos, movimentos de braços, pernas, pescoço.',
    'wellness',
    'exercise',
    '{
        "type": "OBJECT",
        "properties": {
            "body_part": {
                "type": "STRING",
                "description": "Parte do corpo",
                "enum": ["full_body", "arms", "legs", "neck", "back", "hands"]
            },
            "duration_minutes": {
                "type": "INTEGER",
                "description": "Duração em minutos"
            },
            "intensity": {
                "type": "STRING",
                "description": "Intensidade",
                "enum": ["gentle", "moderate"]
            }
        },
        "required": ["body_part"]
    }'::jsonb,
    true,
    'internal',
    60,
    false,
    'Use para manter mobilidade, após acordar, ou quando paciente sentir corpo travado.',
    '["exercício sentado", "alongamento", "mexer o corpo", "ginástica na cadeira"]'::jsonb,
    '["exercício", "alongamento", "mobilidade", "saúde"]'::jsonb
),

-- 21. sleep_stories
(
    'sleep_stories',
    'Histórias para Dormir',
    'Conta histórias calmas e relaxantes especialmente projetadas para induzir o sono. Voz suave, ritmo lento.',
    'wellness',
    'sleep',
    '{
        "type": "OBJECT",
        "properties": {
            "story_theme": {
                "type": "STRING",
                "description": "Tema da história",
                "enum": ["nature", "journey", "countryside", "ocean", "garden", "clouds"]
            },
            "include_breathing": {
                "type": "BOOLEAN",
                "description": "Incluir pausas para respiração"
            }
        },
        "required": ["story_theme"]
    }'::jsonb,
    true,
    'internal',
    65,
    false,
    'Use na hora de dormir, para insônia, ou quando paciente não consegue relaxar.',
    '["história pra dormir", "me ajuda a dormir", "tô com insônia", "conta algo pra eu relaxar"]'::jsonb,
    '["sono", "relaxamento", "insônia", "histórias"]'::jsonb
),

-- 22. gratitude_journal
(
    'gratitude_journal',
    'Diário de Gratidão',
    'Guia prática de gratidão diária. Paciente diz 3 coisas boas do dia. Armazena para reler depois.',
    'wellness',
    'mental_health',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação",
                "enum": ["add_entry", "read_today", "read_week", "read_random"]
            },
            "gratitude_items": {
                "type": "STRING",
                "description": "Coisas pelas quais está grato"
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    55,
    false,
    'Use para rotina positiva, combater negatividade, ou antes de dormir.',
    '["gratidão do dia", "coisas boas de hoje", "pelo que sou grato", "diário de gratidão"]'::jsonb,
    '["gratidão", "bem-estar", "positividade", "saúde mental"]'::jsonb
),

-- 23. motivational_quotes
(
    'motivational_quotes',
    'Frases Motivacionais',
    'Compartilha citações inspiradoras e motivacionais de grandes pensadores, santos, e figuras históricas.',
    'wellness',
    'motivation',
    '{
        "type": "OBJECT",
        "properties": {
            "theme": {
                "type": "STRING",
                "description": "Tema da citação",
                "enum": ["general", "courage", "love", "faith", "wisdom", "perseverance", "happiness"]
            },
            "author_type": {
                "type": "STRING",
                "description": "Tipo de autor",
                "enum": ["any", "saints", "philosophers", "writers", "brazilian"]
            }
        },
        "required": []
    }'::jsonb,
    true,
    'internal',
    45,
    false,
    'Use de manhã, quando paciente estiver desanimado, ou precisar de inspiração.',
    '["uma frase bonita", "me inspira", "pensamento do dia", "frase de sabedoria"]'::jsonb,
    '["motivação", "inspiração", "citações", "sabedoria"]'::jsonb
);

-- =====================================================
-- CATEGORIA 5: SOCIAL E FAMÍLIA (4 ferramentas)
-- =====================================================

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES

-- 24. voice_capsule
(
    'voice_capsule',
    'Cápsula de Voz',
    'Grava mensagens de voz para enviar à família. Pode ser enviada imediatamente ou agendada para datas especiais.',
    'social',
    'family',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação",
                "enum": ["record", "play_back", "send_now", "schedule", "list"]
            },
            "recipient": {
                "type": "STRING",
                "description": "Nome do destinatário (familiar)"
            },
            "scheduled_date": {
                "type": "STRING",
                "description": "Data para envio (formato: YYYY-MM-DD)"
            },
            "occasion": {
                "type": "STRING",
                "description": "Ocasião",
                "enum": ["birthday", "holiday", "just_because", "anniversary", "encouragement"]
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    65,
    false,
    'Use quando paciente quiser mandar mensagem para família, ou em datas especiais.',
    '["grava uma mensagem", "manda um recado pro meu filho", "mensagem de aniversário", "quero falar com minha filha"]'::jsonb,
    '["família", "mensagem", "voz", "conexão"]'::jsonb
),

-- 25. birthday_reminder
(
    'birthday_reminder',
    'Lembrete de Aniversários',
    'Gerencia aniversários de familiares e amigos. Avisa com antecedência e sugere mensagens.',
    'social',
    'family',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação",
                "enum": ["check_today", "check_week", "check_month", "add", "list_all"]
            },
            "person_name": {
                "type": "STRING",
                "description": "Nome da pessoa"
            },
            "date": {
                "type": "STRING",
                "description": "Data de aniversário (DD/MM)"
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    50,
    false,
    'Use de manhã para verificar aniversários, ou quando paciente quiser saber datas.',
    '["aniversário hoje?", "quem faz aniversário?", "quando é o aniversário do meu neto?"]'::jsonb,
    '["aniversário", "família", "datas", "lembretes"]'::jsonb
),

-- 26. family_tree_explorer
(
    'family_tree_explorer',
    'Explorar Árvore Genealógica',
    'Navega pela árvore genealógica do paciente. Conta histórias sobre ancestrais e parentes.',
    'social',
    'family',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação",
                "enum": ["explore", "add_person", "add_story", "view_tree", "find_relation"]
            },
            "person_name": {
                "type": "STRING",
                "description": "Nome da pessoa"
            },
            "relation": {
                "type": "STRING",
                "description": "Relação (ex: avó materna, tio paterno)"
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    45,
    false,
    'Use para reminiscência familiar, conectar com raízes, ou preservar memórias.',
    '["minha árvore genealógica", "conta sobre meu avô", "quem era minha bisavó?"]'::jsonb,
    '["família", "genealogia", "memória", "história"]'::jsonb
),

-- 27. photo_slideshow
(
    'photo_slideshow',
    'Apresentação de Fotos',
    'Mostra fotos antigas do paciente e família com narração. Ótimo para reminiscência e conexão emocional.',
    'social',
    'family',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação",
                "enum": ["start", "pause", "next", "previous", "stop", "comment"]
            },
            "album": {
                "type": "STRING",
                "description": "Álbum de fotos",
                "enum": ["childhood", "wedding", "family", "travels", "career", "recent", "all"]
            },
            "with_music": {
                "type": "BOOLEAN",
                "description": "Incluir música de fundo"
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    55,
    false,
    'Use para reminiscência, quando paciente quiser ver fotos, ou em momentos de saudade.',
    '["mostra minhas fotos", "fotos do casamento", "quero ver fotos antigas", "álbum de família"]'::jsonb,
    '["fotos", "família", "memória", "reminiscência"]'::jsonb
);

-- =====================================================
-- CATEGORIA 6: UTILIDADES DIÁRIAS (3 ferramentas)
-- =====================================================

INSERT INTO available_tools (
    name, display_name, description, category, subcategory,
    parameters, enabled, handler_type, priority, is_critical,
    usage_hint, example_prompts, tags
) VALUES

-- 28. weather_chat
(
    'weather_chat',
    'Conversa sobre o Tempo',
    'Informa previsão do tempo de forma conversacional. Dá dicas de vestuário e atividades baseadas no clima.',
    'utility',
    'daily',
    '{
        "type": "OBJECT",
        "properties": {
            "location": {
                "type": "STRING",
                "description": "Cidade (usa localização do paciente se vazio)"
            },
            "forecast_type": {
                "type": "STRING",
                "description": "Tipo de previsão",
                "enum": ["now", "today", "tomorrow", "week"]
            }
        },
        "required": []
    }'::jsonb,
    true,
    'internal',
    50,
    false,
    'Use de manhã, quando paciente perguntar do tempo, ou antes de sair.',
    '["como tá o tempo?", "vai chover?", "previsão de amanhã", "preciso de casaco?"]'::jsonb,
    '["tempo", "clima", "previsão", "utilidade"]'::jsonb
),

-- 29. cooking_recipes
(
    'cooking_recipes',
    'Receitas Culinárias',
    'Compartilha receitas simples e tradicionais. Pode guiar passo a passo. Foca em receitas da culinária brasileira.',
    'utility',
    'cooking',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação",
                "enum": ["search", "start_recipe", "next_step", "repeat_step", "list_ingredients"]
            },
            "dish_type": {
                "type": "STRING",
                "description": "Tipo de prato",
                "enum": ["main", "dessert", "soup", "salad", "drink", "snack"]
            },
            "recipe_name": {
                "type": "STRING",
                "description": "Nome da receita"
            },
            "difficulty": {
                "type": "STRING",
                "description": "Dificuldade",
                "enum": ["easy", "medium"]
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    45,
    false,
    'Use quando paciente quiser cozinhar, pedir receitas, ou relembrar pratos de família.',
    '["receita de bolo", "como faz arroz?", "receita da vovó", "o que cozinhar hoje?"]'::jsonb,
    '["receitas", "culinária", "comida", "cozinha"]'::jsonb
),

-- 30. voice_diary
(
    'voice_diary',
    'Diário de Voz',
    'Permite gravar pensamentos e reflexões do dia. Organiza por data e permite ouvir entradas anteriores.',
    'utility',
    'personal',
    '{
        "type": "OBJECT",
        "properties": {
            "action": {
                "type": "STRING",
                "description": "Ação",
                "enum": ["record", "play_today", "play_date", "play_random", "list_recent"]
            },
            "date": {
                "type": "STRING",
                "description": "Data específica (YYYY-MM-DD)"
            },
            "tag": {
                "type": "STRING",
                "description": "Tag para categorizar",
                "enum": ["thought", "memory", "dream", "gratitude", "worry", "plan"]
            }
        },
        "required": ["action"]
    }'::jsonb,
    true,
    'internal',
    55,
    false,
    'Use quando paciente quiser registrar pensamentos, contar algo importante, ou manter memórias.',
    '["quero gravar no diário", "anota isso pra mim", "meu pensamento de hoje", "ouve o que eu disse ontem"]'::jsonb,
    '["diário", "voz", "memória", "registro"]'::jsonb
);

-- =====================================================
-- CAPACIDADES RELACIONADAS
-- =====================================================

INSERT INTO eva_capabilities (
    capability_name, capability_type, description, short_description,
    related_tools, when_to_use, when_not_to_use, example_queries, prompt_priority
) VALUES
(
    'entretenimento_musical',
    'skill',
    'Capacidade de tocar músicas, rádios e sons relaxantes para o paciente.',
    'Posso tocar músicas, rádio e sons da natureza para você.',
    '["play_nostalgic_music", "play_radio_station", "nature_sounds", "religious_content"]'::jsonb,
    'Quando o paciente quiser ouvir música, relaxar, ou precisar de conforto sonoro.',
    'Durante conversas importantes ou avaliações clínicas.',
    '["toca uma música", "liga a rádio", "som pra relaxar"]'::jsonb,
    80
),
(
    'jogos_cognitivos',
    'skill',
    'Capacidade de jogar quiz, memória, palavras e exercícios mentais com o paciente.',
    'Posso jogar quiz, memória e exercícios mentais com você.',
    '["play_trivia_game", "memory_game", "word_association", "brain_training", "complete_the_lyrics", "riddles_and_jokes"]'::jsonb,
    'Quando o paciente quiser se distrair, exercitar a mente, ou estiver entediado.',
    'Quando paciente estiver cansado ou em crise emocional.',
    '["vamos jogar", "exercício de memória", "conta uma piada"]'::jsonb,
    70
),
(
    'contar_historias',
    'skill',
    'Capacidade de contar e criar histórias, ler notícias, e guiar reminiscência.',
    'Posso contar histórias, ler notícias e ajudar você a relembrar o passado.',
    '["story_generator", "reminiscence_therapy", "biography_writer", "read_newspaper", "daily_horoscope"]'::jsonb,
    'Quando paciente quiser ouvir histórias, se informar, ou falar do passado.',
    'Durante avaliações clínicas formais.',
    '["conta uma história", "lê as notícias", "vamos falar da minha infância"]'::jsonb,
    65
),
(
    'bem_estar_relaxamento',
    'skill',
    'Capacidade de guiar meditação, respiração, exercícios e práticas de bem-estar.',
    'Posso guiar meditação, respiração e exercícios para seu bem-estar.',
    '["guided_meditation", "breathing_exercises", "chair_exercises", "sleep_stories", "gratitude_journal", "motivational_quotes"]'::jsonb,
    'Quando paciente estiver ansioso, precisar relaxar, não conseguir dormir, ou quiser cuidar da saúde.',
    'Em emergências ou crises que requerem intervenção profissional.',
    '["medita comigo", "exercício de respiração", "história pra dormir"]'::jsonb,
    75
),
(
    'conexao_familiar',
    'skill',
    'Capacidade de ajudar a manter conexão com família através de mensagens e memórias.',
    'Posso ajudar você a mandar mensagens e manter conexão com sua família.',
    '["voice_capsule", "birthday_reminder", "family_tree_explorer", "photo_slideshow"]'::jsonb,
    'Quando paciente quiser falar com família, em datas especiais, ou para preservar memórias.',
    'Quando paciente estiver muito emotivo ou em crise.',
    '["manda mensagem pro meu filho", "mostra fotos da família", "aniversário de quem?"]'::jsonb,
    70
)
ON CONFLICT (capability_name) DO UPDATE SET
    description = EXCLUDED.description,
    related_tools = EXCLUDED.related_tools,
    updated_at = NOW();

-- =====================================================
-- PERMISSÕES POR PERSONA
-- =====================================================

-- Companion tem acesso a TODAS as ferramentas de entretenimento
INSERT INTO persona_tool_permissions (persona_code, tool_name, permission_type, max_uses_per_day, requires_reason)
SELECT 'companion', name, 'allowed',
    CASE
        WHEN category = 'wellness' THEN 10
        WHEN category = 'games' THEN 20
        ELSE NULL
    END,
    FALSE
FROM available_tools
WHERE category IN ('entertainment', 'wellness', 'social', 'utility')
ON CONFLICT DO NOTHING;

-- Clinical tem acesso limitado (apenas relaxamento e bem-estar)
INSERT INTO persona_tool_permissions (persona_code, tool_name, permission_type, max_uses_per_day, requires_reason)
SELECT 'clinical', name, 'allowed_with_limits', 3, TRUE
FROM available_tools
WHERE name IN ('breathing_exercises', 'guided_meditation', 'motivational_quotes')
ON CONFLICT DO NOTHING;

-- Emergency NÃO tem acesso a entretenimento
INSERT INTO persona_tool_permissions (persona_code, tool_name, permission_type)
SELECT 'emergency', name, 'prohibited'
FROM available_tools
WHERE category IN ('entertainment', 'games', 'social')
ON CONFLICT DO NOTHING;

-- Educator tem acesso a jogos cognitivos
INSERT INTO persona_tool_permissions (persona_code, tool_name, permission_type, max_uses_per_day)
SELECT 'educator', name, 'allowed', 15
FROM available_tools
WHERE name IN ('play_trivia_game', 'memory_game', 'word_association', 'brain_training')
ON CONFLICT DO NOTHING;

-- =====================================================
-- VERIFICAÇÃO FINAL
-- =====================================================

DO $$
DECLARE
    entertainment_count INTEGER;
    capability_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO entertainment_count
    FROM available_tools
    WHERE category IN ('entertainment', 'wellness', 'social', 'utility')
    AND name LIKE '%' -- todas as novas ferramentas
    AND created_at > NOW() - INTERVAL '1 minute';

    SELECT COUNT(*) INTO capability_count
    FROM eva_capabilities
    WHERE capability_name LIKE '%entretenimento%'
       OR capability_name LIKE '%jogos%'
       OR capability_name LIKE '%historias%'
       OR capability_name LIKE '%bem_estar%'
       OR capability_name LIKE '%familiar%';

    RAISE NOTICE '✅ Entertainment Tools Migration Complete:';
    RAISE NOTICE '   - % ferramentas de entretenimento registradas', entertainment_count;
    RAISE NOTICE '   - % capacidades definidas', capability_count;
END $$;
