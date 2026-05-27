-- Created at: 2026-05-27T00:00:00Z

-- @UP
CREATE TABLE IF NOT EXISTS users (
    user_id    TEXT      PRIMARY KEY,
    ssoid      TEXT      NOT NULL UNIQUE,
    email      TEXT,
    name       TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_ssoid ON users(ssoid);

-- @DOWN
DROP INDEX IF EXISTS idx_users_ssoid;
DROP TABLE IF EXISTS users;
