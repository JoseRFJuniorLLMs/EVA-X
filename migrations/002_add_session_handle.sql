-- Migration to add session handle column if not exists (example of follow-up migration)

DO $$ 
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='agendamentos' AND column_name='gemini_session_handle') THEN
        ALTER TABLE agendamentos ADD COLUMN gemini_session_handle TEXT;
    END IF;
END $$;
