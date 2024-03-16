-- +goose Up
-- +goose StatementBegin
CREATE TABLE `series` (
  `id` BIGINT UNSIGNED PRIMARY KEY,
  `name` VARCAHR(255) NOT NULL,
  `poster_path` TEXT,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIME
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE `series`;
-- +goose StatementEnd
