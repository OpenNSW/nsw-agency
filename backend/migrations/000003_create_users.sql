-- Created at: 2026-05-27T00:00:00Z

-- @UP
CREATE TABLE IF NOT EXISTS users (
    user_id    TEXT      PRIMARY KEY,
    ssoid      TEXT      UNIQUE,
    email      TEXT,
    name       TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- @DOWN
DROP TABLE IF EXISTS users;
