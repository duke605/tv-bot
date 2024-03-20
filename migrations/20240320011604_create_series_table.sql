-- +goose Up
-- +goose StatementBegin
CREATE TABLE `series` (
  `id` BIGINT UNSIGNED NOT NULL PRIMARY KEY,
  `next_episode_air_date` TIMESTAMP,
  `data` BLOB NOT NULL,
  `last_fetched_at` TIMESTAMP NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE `series`;
-- +goose StatementEnd
