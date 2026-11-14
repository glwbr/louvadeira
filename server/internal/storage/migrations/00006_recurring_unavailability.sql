-- +goose Up
-- NOTE: A repeating absence covers only matching days inside [start_date, end_date]: every week on weekday, or
-- the week_of_month-th weekday of each month. An open-ended one stores end_date 9999-12-31.
ALTER TABLE unavailabilities ADD COLUMN repeat TEXT NOT NULL DEFAULT 'none' CHECK (repeat IN ('none', 'weekly', 'monthly'));
ALTER TABLE unavailabilities ADD COLUMN weekday INTEGER CHECK (weekday IS NULL OR weekday BETWEEN 0 AND 6);
ALTER TABLE unavailabilities ADD COLUMN week_of_month INTEGER CHECK (week_of_month IS NULL OR week_of_month BETWEEN 1 AND 4);

-- +goose Down
ALTER TABLE unavailabilities DROP COLUMN week_of_month;
ALTER TABLE unavailabilities DROP COLUMN weekday;
ALTER TABLE unavailabilities DROP COLUMN repeat;
