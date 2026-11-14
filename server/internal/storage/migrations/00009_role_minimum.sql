-- +goose Up
-- NOTE: min_slots is how few people a role can go down to before the leaders are warned; NULL means every slot.
ALTER TABLE roles ADD COLUMN min_slots INTEGER CHECK (min_slots IS NULL OR min_slots BETWEEN 0 AND 19);

-- +goose Down
ALTER TABLE roles DROP COLUMN min_slots;
