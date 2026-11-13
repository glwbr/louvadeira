-- +goose Up
CREATE TABLE people (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL CHECK (length(trim(name)) > 0),
    phone       TEXT UNIQUE CHECK (phone IS NULL OR (length(phone) BETWEEN 8 AND 15 AND phone NOT GLOB '*[^0-9]*')),
    is_admin    INTEGER NOT NULL DEFAULT 0 CHECK (is_admin IN (0, 1)),
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    archived_at TEXT
) STRICT;

CREATE TABLE ministries (
    id            INTEGER PRIMARY KEY,
    name          TEXT NOT NULL UNIQUE CHECK (length(trim(name)) > 0),
    rotation_unit TEXT NOT NULL DEFAULT 'week' CHECK (rotation_unit IN ('week', 'service')),
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    archived_at   TEXT
) STRICT;

CREATE TABLE roles (
    id          INTEGER PRIMARY KEY,
    ministry_id INTEGER NOT NULL REFERENCES ministries (id) ON DELETE CASCADE,
    name        TEXT NOT NULL CHECK (length(trim(name)) > 0),
    slots       INTEGER NOT NULL DEFAULT 1 CHECK (slots BETWEEN 1 AND 20),
    position    INTEGER NOT NULL DEFAULT 0,
    UNIQUE (ministry_id, name),
    UNIQUE (id, ministry_id)
) STRICT;

CREATE TABLE ministry_members (
    ministry_id INTEGER NOT NULL REFERENCES ministries (id) ON DELETE CASCADE,
    person_id   INTEGER NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    is_leader   INTEGER NOT NULL DEFAULT 0 CHECK (is_leader IN (0, 1)),
    PRIMARY KEY (ministry_id, person_id)
) STRICT;

-- INVARIANT: The composite keys guarantee a member only holds roles of the ministry they belong to.
CREATE TABLE member_roles (
    ministry_id INTEGER NOT NULL,
    person_id   INTEGER NOT NULL,
    role_id     INTEGER NOT NULL,
    fixed       INTEGER NOT NULL DEFAULT 0 CHECK (fixed IN (0, 1)),
    PRIMARY KEY (person_id, role_id),
    FOREIGN KEY (ministry_id, person_id) REFERENCES ministry_members (ministry_id, person_id) ON DELETE CASCADE,
    FOREIGN KEY (role_id, ministry_id) REFERENCES roles (id, ministry_id) ON DELETE CASCADE
) STRICT;

CREATE TABLE services (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL CHECK (length(trim(name)) > 0),
    weekday    INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_time TEXT NOT NULL CHECK (start_time GLOB '[0-2][0-9]:[0-5][0-9]'),
    active     INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE ministry_services (
    ministry_id INTEGER NOT NULL REFERENCES ministries (id) ON DELETE CASCADE,
    service_id  INTEGER NOT NULL REFERENCES services (id) ON DELETE CASCADE,
    PRIMARY KEY (ministry_id, service_id)
) STRICT;

CREATE TABLE login_tokens (
    token_hash TEXT PRIMARY KEY,
    person_id  INTEGER NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    used_at    TEXT
) STRICT;

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    person_id  INTEGER NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
) STRICT;

-- +goose Down
DROP TABLE sessions;
DROP TABLE login_tokens;
DROP TABLE ministry_services;
DROP TABLE services;
DROP TABLE member_roles;
DROP TABLE ministry_members;
DROP TABLE roles;
DROP TABLE ministries;
DROP TABLE people;
