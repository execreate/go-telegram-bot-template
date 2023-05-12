-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE `telegram_users` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`first_name` text,`last_name` text,`username` text,`language_code` text,PRIMARY KEY (`id`));
CREATE INDEX `idx_telegram_users_deleted_at` ON `telegram_users`(`deleted_at`);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE `telegram_users`;
-- +goose StatementEnd
