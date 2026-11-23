-- +goose Up
-- NOTE: An invitation whose send failed is kept as unsent, so the swap waits for it and it is tried again, even when
-- other people were invited.
ALTER TABLE cover_invites ADD COLUMN sent INTEGER NOT NULL DEFAULT 1 CHECK (sent IN (0, 1));

-- +goose Down
ALTER TABLE cover_invites DROP COLUMN sent;
