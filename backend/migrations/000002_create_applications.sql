-- @UP
CREATE TABLE IF NOT EXISTS applications (
    task_id                 TEXT         PRIMARY KEY,
    task_code               VARCHAR(100) NOT NULL,
    consignment_id          TEXT         NOT NULL,
    service_url             VARCHAR(512) NOT NULL,
    data                    TEXT,
    reviewer_response       TEXT,
    status                  VARCHAR(50)  NOT NULL DEFAULT 'PENDING',
    agency_feedback_history TEXT,
    reviewed_at             TIMESTAMP,
    created_at              TIMESTAMP     NOT NULL,
    updated_at              TIMESTAMP     NOT NULL,
    CONSTRAINT fk_applications_consignment_id
        FOREIGN KEY (consignment_id) REFERENCES consignments(id)
        ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_applications_consignment_id ON applications(consignment_id);

-- @DOWN
DROP INDEX IF EXISTS idx_applications_consignment_id;
DROP TABLE IF EXISTS applications;
