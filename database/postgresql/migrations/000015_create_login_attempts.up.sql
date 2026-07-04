CREATE TABLE IF NOT EXISTS login_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone VARCHAR(15) NOT NULL,
    ip_address VARCHAR(45),
    status VARCHAR(20) NOT NULL DEFAULT 'SENT',
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_login_attempts_phone
    ON login_attempts(phone);

CREATE INDEX IF NOT EXISTS idx_login_attempts_status
    ON login_attempts(status);

CREATE INDEX IF NOT EXISTS idx_login_attempts_created_at
    ON login_attempts(created_at DESC);
