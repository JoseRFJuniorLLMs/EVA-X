-- ============================================================================
-- MIGRATION: Adiciona suporte a idioma por idoso
-- Sistema internacional - EVA detecta idioma do usuário
-- Baseado nos idiomas suportados pelo Gemini Live API
-- ============================================================================

-- Adicionar coluna idioma na tabela idosos
ALTER TABLE idosos ADD COLUMN IF NOT EXISTS idioma VARCHAR(10) DEFAULT 'pt-BR';

-- Criar índice para consultas rápidas
CREATE INDEX IF NOT EXISTS idx_idosos_idioma ON idosos(idioma);

-- Comentário
COMMENT ON COLUMN idosos.idioma IS 'Idioma preferido do idoso - Gemini Live API supported languages';

-- ============================================================================
-- IDIOMAS SUPORTADOS PELO GEMINI LIVE API (30 idiomas):
-- ============================================================================
--
-- PORTUGUÊS:
--   pt-BR = Português (Brasil) - DEFAULT
--
-- INGLÊS:
--   en-US = English (United States)
--   en-GB = English (United Kingdom)*
--   en-AU = English (Australia)*
--   en-IN = English (India)
--
-- ESPANHOL:
--   es-ES = Español (España)*
--   es-US = Español (Estados Unidos)
--
-- FRANCÊS:
--   fr-FR = Français (France)
--   fr-CA = Français (Canada)*
--
-- ALEMÃO:
--   de-DE = Deutsch (Deutschland)
--
-- ITALIANO:
--   it-IT = Italiano (Italia)
--
-- ASIÁTICOS:
--   ja-JP = 日本語 (Japanese)
--   ko-KR = 한국어 (Korean)
--   cmn-CN = 中文 (Mandarin Chinese)*
--   th-TH = ไทย (Thai)
--   vi-VN = Tiếng Việt (Vietnamese)
--   id-ID = Bahasa Indonesia
--
-- INDIANOS:
--   hi-IN = हिन्दी (Hindi)
--   bn-IN = বাংলা (Bengali)
--   gu-IN = ગુજરાતી (Gujarati)*
--   kn-IN = ಕನ್ನಡ (Kannada)*
--   ml-IN = മലയാളം (Malayalam)*
--   mr-IN = मराठी (Marathi)
--   ta-IN = தமிழ் (Tamil)
--   te-IN = తెలుగు (Telugu)
--
-- OUTROS:
--   ar-XA = العربية (Arabic)
--   nl-NL = Nederlands (Dutch)
--   pl-PL = Polski (Polish)
--   ru-RU = Русский (Russian)
--   tr-TR = Türkçe (Turkish)
--
-- * Nota: Idiomas marcados com * não estão disponíveis para Native Audio
-- ============================================================================

SELECT 'Migration 025_idioma_idoso complete!' AS status;
