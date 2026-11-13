-- +goose Up
CREATE TABLE unavailabilities (
    id         INTEGER PRIMARY KEY,
    person_id  INTEGER NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    start_date TEXT NOT NULL CHECK (date(start_date) IS start_date),
    end_date   TEXT NOT NULL CHECK (date(end_date) IS end_date AND end_date >= start_date),
    note       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE INDEX unavailabilities_by_dates ON unavailabilities (start_date, end_date);

CREATE TABLE schedules (
    id           INTEGER PRIMARY KEY,
    ministry_id  INTEGER NOT NULL REFERENCES ministries (id) ON DELETE CASCADE,
    month        TEXT NOT NULL CHECK (month GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]'),
    status       TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    generated_at TEXT NOT NULL,
    published_at TEXT,
    UNIQUE (ministry_id, month)
) STRICT;

-- NOTE: person_id is NULL for a slot nobody could fill.
CREATE TABLE assignments (
    id           INTEGER PRIMARY KEY,
    schedule_id  INTEGER NOT NULL REFERENCES schedules (id) ON DELETE CASCADE,
    service_id   INTEGER NOT NULL REFERENCES services (id) ON DELETE CASCADE,
    service_date TEXT NOT NULL CHECK (date(service_date) IS service_date),
    role_id      INTEGER NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    slot         INTEGER NOT NULL CHECK (slot >= 0),
    person_id    INTEGER REFERENCES people (id) ON DELETE SET NULL,
    UNIQUE (schedule_id, service_date, service_id, role_id, slot)
) STRICT;

CREATE INDEX assignments_by_person ON assignments (person_id, service_date);

-- +goose Down
DROP TABLE assignments;
DROP TABLE schedules;
DROP TABLE unavailabilities;
