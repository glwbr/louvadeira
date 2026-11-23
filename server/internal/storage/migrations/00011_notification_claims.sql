-- +goose Up
-- NOTE: A scheduled message is recorded as sending before it goes out and as sent once WhatsApp takes it. A record
-- still sending when the server starts is from a send a restart or a power cut interrupted, so it is dropped and the
-- message goes out again.
ALTER TABLE notifications ADD COLUMN sending INTEGER NOT NULL DEFAULT 0 CHECK (sending IN (0, 1));

-- +goose Down
ALTER TABLE notifications DROP COLUMN sending;
