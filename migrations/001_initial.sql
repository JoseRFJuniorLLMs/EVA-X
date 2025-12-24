-- Initial database schema for EVA-Mind

CREATE TABLE idosos (
    id SERIAL PRIMARY KEY,
    nome TEXT NOT NULL,
    telefone TEXT NOT NULL,
    nivel_cognitivo TEXT,
    limitacoes_auditivas BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE agendamentos (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER REFERENCES idosos(id),
    nome_idoso TEXT,
    telefone TEXT,
    horario TIMESTAMP NOT NULL,
    remedios TEXT,
    status TEXT DEFAULT 'pendente',
    tentativas_realizadas INTEGER DEFAULT 0,
    call_sid TEXT,
    gemini_session_handle TEXT,
    ultima_interacao_estado JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE historico_ligacoes (
    id SERIAL PRIMARY KEY,
    agendamento_id INTEGER REFERENCES agendamentos(id),
    idoso_id INTEGER REFERENCES idosos(id),
    call_sid TEXT,
    status TEXT,
    inicio TIMESTAMP,
    fim TIMESTAMP,
    qualidade_audio TEXT,
    interrupcoes_detectadas INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);
