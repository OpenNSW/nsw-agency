-- @UP
CREATE TABLE IF NOT EXISTS consignments (
    id         TEXT        PRIMARY KEY,
    status     VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP   NOT NULL,
    updated_at TIMESTAMP   NOT NULL
);

-- @DOWN
DROP TABLE IF EXISTS consignments;
