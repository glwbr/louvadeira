-- +goose Up
ALTER TABLE unavailabilities ADD COLUMN reason TEXT NOT NULL DEFAULT 'outro'
    CHECK (reason IN ('viagem', 'trabalho', 'saude', 'familia', 'estudos', 'outro'));
ALTER TABLE unavailabilities DROP COLUMN note;

-- NOTE: communion_week is the nth occurrence of the service's weekday in the month (3 = third Sunday).
ALTER TABLE services ADD COLUMN communion_week INTEGER CHECK (communion_week IS NULL OR communion_week BETWEEN 1 AND 4);

-- +goose Down
ALTER TABLE services DROP COLUMN communion_week;
ALTER TABLE unavailabilities ADD COLUMN note TEXT NOT NULL DEFAULT '';
ALTER TABLE unavailabilities DROP COLUMN reason;
