-- +goose Up
-- NOTE: source ties an absence to the availability poll answer that created it ("enquete:<message id>"), so a
-- changed answer replaces exactly those days. Absences entered in the app or by command have no source.
ALTER TABLE unavailabilities ADD COLUMN source TEXT;
CREATE INDEX unavailabilities_by_source ON unavailabilities (source) WHERE source IS NOT NULL;

-- +goose Down
DROP INDEX unavailabilities_by_source;
ALTER TABLE unavailabilities DROP COLUMN source;
