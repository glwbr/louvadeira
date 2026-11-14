-- +goose Up
-- NOTE: A poll in a group takes votes from many people, so each voter's vote waits on its own. voter is the phone
-- WhatsApp reported for the vote, empty when it reported none.
CREATE TABLE pending_votes_by_voter (
    poll_id     TEXT NOT NULL,
    voter       TEXT NOT NULL DEFAULT '',
    selected    TEXT NOT NULL,
    from_me     INTEGER NOT NULL CHECK (from_me IN (0, 1)),
    received_at TEXT NOT NULL,
    PRIMARY KEY (poll_id, voter)
) STRICT;
INSERT INTO pending_votes_by_voter (poll_id, selected, from_me, received_at)
    SELECT poll_id, selected, from_me, received_at FROM pending_votes;
DROP TABLE pending_votes;
ALTER TABLE pending_votes_by_voter RENAME TO pending_votes;

-- +goose Down
CREATE TABLE pending_votes_by_poll (
    poll_id     TEXT PRIMARY KEY,
    selected    TEXT NOT NULL,
    from_me     INTEGER NOT NULL CHECK (from_me IN (0, 1)),
    received_at TEXT NOT NULL
) STRICT;
INSERT OR REPLACE INTO pending_votes_by_poll (poll_id, selected, from_me, received_at)
    SELECT poll_id, selected, from_me, received_at FROM pending_votes ORDER BY received_at;
DROP TABLE pending_votes;
ALTER TABLE pending_votes_by_poll RENAME TO pending_votes;
