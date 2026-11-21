-- 008_admin_and_activity.sql
-- Roles, account suspension, case-insensitive unique emails, and an activity log.

ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'user';
ALTER TABLE users ADD COLUMN IF NOT EXISTS disabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_role_check') THEN
        ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('user', 'admin'));
    END IF;
END $$;

-- "A@x.com" and "a@x.com" are the same account.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users (LOWER(email));

-- One row per recorded event. No request bodies, tokens or passwords are ever
-- stored. `subject` is a one-way hash of the email used in a sign-in attempt,
-- so repeated failures can be counted without keeping the address itself.
CREATE TABLE IF NOT EXISTS activity_logs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    request_id VARCHAR(64),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(64) NOT NULL,
    method VARCHAR(10) NOT NULL,
    path VARCHAR(255) NOT NULL,
    status SMALLINT NOT NULL,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    ip VARCHAR(64),
    user_agent VARCHAR(255),
    subject VARCHAR(64)
);

CREATE INDEX IF NOT EXISTS idx_activity_created_at ON activity_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_user ON activity_logs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_action ON activity_logs (action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_ip ON activity_logs (ip, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_subject ON activity_logs (subject, created_at DESC);
