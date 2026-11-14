-- +goose Up
-- NOTE: A vote waits a few seconds before it counts, in case the person is still changing it. Keeping it here means a
-- restart in those seconds does not lose it: the server applies what is left when it comes back.
CREATE TABLE pending_votes (
    poll_id     TEXT PRIMARY KEY,
    selected    TEXT NOT NULL,
    from_me     INTEGER NOT NULL CHECK (from_me IN (0, 1)),
    received_at TEXT NOT NULL
) STRICT;

-- +goose Down
DROP TABLE pending_votes;
