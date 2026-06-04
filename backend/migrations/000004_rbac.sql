-- Created at: 2026-06-04T00:00:00Z

-- @UP
CREATE TABLE IF NOT EXISTS roles (
    id          TEXT      PRIMARY KEY,
    name        TEXT      NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_name ON roles(name);

CREATE TABLE IF NOT EXISTS user_roles (
    id         TEXT      PRIMARY KEY,
    user_id    TEXT      NOT NULL REFERENCES users(user_id),
    role_id    TEXT      NOT NULL REFERENCES roles(id),
    assigned_at TIMESTAMP NOT NULL,
    UNIQUE(user_id, role_id)
);
CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);

-- @DOWN
DROP INDEX IF EXISTS idx_user_roles_user_id;
DROP TABLE IF EXISTS user_roles;
DROP INDEX IF EXISTS idx_roles_name;
DROP TABLE IF EXISTS roles;
