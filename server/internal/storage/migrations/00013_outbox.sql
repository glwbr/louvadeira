-- +goose Up
-- NOTE: Messages about a change already saved (a reply to a vote, a swap announced in the group, a notice to the
-- leaders) are recorded before they are sent, so a dropped connection or a restart does not lose them: each tick tries
-- the pending ones again, in order per recipient. The rows also keep a record of what the bot sent.
CREATE TABLE outbox (
    id         INTEGER PRIMARY KEY,
    recipient  TEXT NOT NULL,
    kind       TEXT NOT NULL CHECK (kind IN ('text', 'mention')),
    text       TEXT NOT NULL,
    mentions   TEXT NOT NULL DEFAULT '[]',
    status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'dropped')),
    attempts   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    sent_at    TEXT
) STRICT;
CREATE INDEX outbox_pending ON outbox (recipient, id) WHERE status = 'pending';

-- +goose Down
DROP TABLE outbox;
