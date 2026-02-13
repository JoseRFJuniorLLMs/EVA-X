-- Migration: 023_historico_medicamentos
-- Cria tabela para registrar confirmações de medicamentos por voz
-- Usada por: ConfirmMedication() em internal/cortex/gemini/actions.go

CREATE TABLE IF NOT EXISTS historico_medicamentos (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    medicamento VARCHAR(255) NOT NULL,
    tomado BOOLEAN DEFAULT TRUE,
    data_hora TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Metadados extras
    fonte VARCHAR(50) DEFAULT 'voz',  -- 'voz', 'visual', 'manual'
    confirmado_por VARCHAR(100),       -- 'idoso', 'cuidador', 'familiar'
    observacoes TEXT,

    -- Auditoria
    criado_em TIMESTAMP DEFAULT NOW(),
    atualizado_em TIMESTAMP DEFAULT NOW()
);

-- Índices para consultas comuns
CREATE INDEX IF NOT EXISTS idx_hist_med_idoso ON historico_medicamentos(idoso_id);
CREATE INDEX IF NOT EXISTS idx_hist_med_data ON historico_medicamentos(data_hora);
CREATE INDEX IF NOT EXISTS idx_hist_med_medicamento ON historico_medicamentos(medicamento);

-- Comentários
COMMENT ON TABLE historico_medicamentos IS 'Histórico de confirmações de medicamentos tomados pelo idoso';
COMMENT ON COLUMN historico_medicamentos.fonte IS 'Origem da confirmação: voz, visual (câmera), manual';
COMMENT ON COLUMN historico_medicamentos.tomado IS 'Se o medicamento foi efetivamente tomado';
