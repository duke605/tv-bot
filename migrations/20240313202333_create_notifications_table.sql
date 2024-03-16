-- +goose Up
-- +goose StatementBegin
CREATE TABLE `notifications` (
  `episode` INT NOT NULL,
  `season` INT NOT NULL,
  `series_id` BIGINT UNSIGNED NOT NULL,
  `discord_message_id` BIGINT UNSIGNED NOT NULL,

  PRIMARY KEY (`episode`, `season`, `series_id`),
  FOREIGN KEY (`series_id`) REFERENCES `series`(`id`)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE `notifications`;
-- +goose StatementEnd
