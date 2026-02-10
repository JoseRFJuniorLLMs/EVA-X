-- Migration: Create device_tokens table
-- Description: Armazena tokens de dispositivos móveis para push notifications
-- Created: 2026-01-23

CREATE TABLE IF NOT EXISTS device_tokens (
    id SERIAL PRIMARY KEY,
    idoso_id INTEGER NOT NULL REFERENCES idosos(id) ON DELETE CASCADE,
    token TEXT NOT NULL,
    platform VARCHAR(20) NOT NULL CHECK (platform IN ('ios', 'android')),
    app_version VARCHAR(50),
    device_model VARCHAR(100),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Índices para performance
    CONSTRAINT unique_token_per_idoso UNIQUE(idoso_id, token)
);

-- Índices
CREATE INDEX idx_device_tokens_idoso_id ON device_tokens(idoso_id);
CREATE INDEX idx_device_tokens_active ON device_tokens(is_active) WHERE is_active = true;
CREATE INDEX idx_device_tokens_platform ON device_tokens(platform);
CREATE INDEX idx_device_tokens_last_used ON device_tokens(last_used_at);

-- Comentários
COMMENT ON TABLE device_tokens IS 'Tokens de dispositivos móveis para push notifications Firebase';
COMMENT ON COLUMN device_tokens.token IS 'Firebase Cloud Messaging (FCM) registration token';
COMMENT ON COLUMN device_tokens.platform IS 'Plataforma do dispositivo: ios ou android';
COMMENT ON COLUMN device_tokens.is_active IS 'Indica se o token ainda está ativo (usuário não desinstalou o app)';
COMMENT ON COLUMN device_tokens.last_used_at IS 'Última vez que o token foi usado com sucesso';
