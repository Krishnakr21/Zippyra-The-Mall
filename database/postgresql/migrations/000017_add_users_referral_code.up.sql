ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_code VARCHAR(6) UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS referred_by UUID REFERENCES users(id);

CREATE INDEX IF NOT EXISTS idx_users_referral_code ON users(referral_code);
