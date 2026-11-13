-- +goose Up
ALTER TABLE ministries ADD COLUMN whatsapp_group TEXT CHECK (whatsapp_group IS NULL OR whatsapp_group GLOB '*@g.us');

ALTER TABLE assignments ADD COLUMN status TEXT NOT NULL DEFAULT 'scheduled'
    CHECK (status IN ('scheduled', 'confirmed', 'declined'));
ALTER TABLE assignments ADD COLUMN responded_at TEXT;

-- NOTE: One row per message the scheduler already sent, so a restart never sends it twice.
CREATE TABLE notifications (
    key     TEXT PRIMARY KEY,
    sent_at TEXT NOT NULL
) STRICT;

CREATE TABLE polls (
    message_id TEXT PRIMARY KEY,
    kind       TEXT NOT NULL CHECK (kind IN ('confirm', 'cover', 'test')),
    person_id  INTEGER REFERENCES people (id) ON DELETE CASCADE,
    options    TEXT NOT NULL,
    created_at TEXT NOT NULL
) STRICT;

CREATE TABLE cover_requests (
    id               INTEGER PRIMARY KEY,
    assignment_id    INTEGER NOT NULL REFERENCES assignments (id) ON DELETE CASCADE,
    absent_person_id INTEGER NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    status           TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'covered', 'uncovered')),
    late             INTEGER NOT NULL DEFAULT 0 CHECK (late IN (0, 1)),
    covered_by       INTEGER REFERENCES people (id) ON DELETE SET NULL,
    created_at       TEXT NOT NULL,
    resolved_at      TEXT
) STRICT;

CREATE TABLE cover_invites (
    request_id  INTEGER NOT NULL REFERENCES cover_requests (id) ON DELETE CASCADE,
    person_id   INTEGER NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    answer      TEXT CHECK (answer IS NULL OR answer IN ('yes', 'no')),
    answered_at TEXT,
    PRIMARY KEY (request_id, person_id)
) STRICT;

-- +goose Down
DROP TABLE cover_invites;
DROP TABLE cover_requests;
DROP TABLE polls;
DROP TABLE notifications;
ALTER TABLE assignments DROP COLUMN responded_at;
ALTER TABLE assignments DROP COLUMN status;
ALTER TABLE ministries DROP COLUMN whatsapp_group;
