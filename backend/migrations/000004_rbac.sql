-- Created at: 2026-06-04T00:00:00Z

-- @UP
CREATE TABLE IF NOT EXISTS roles (
    id          TEXT      PRIMARY KEY,
    name        TEXT      NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS user_roles (
    id          TEXT      PRIMARY KEY,
    user_id     TEXT      NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    role_id     TEXT      NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP NOT NULL,
    UNIQUE(user_id, role_id)
);

-- @DOWN
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
