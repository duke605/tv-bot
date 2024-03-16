-- +goose Up
-- +goose StatementBegin
CREATE TABLE `subscriptions` (
  `series_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (`series_id`, `user_id`)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE `subscriptions`;
-- +goose StatementEnd
