-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

CREATE TABLE `telegram_users`
(
    `id`                                   integer,
    `created_at`                           datetime,
    `updated_at`                           datetime,
    `deleted_at`                           datetime,
    `first_name`                           text,
    `last_name`                            text,
    `username`                             text,
    `language_code`                        text,
    `is_admin`                             numeric,
    `accepted_terms_and_conditions_on`     datetime,
    `accepted_latest_terms_and_conditions` numeric,
    PRIMARY KEY (`id`)
);
CREATE INDEX `idx_telegram_users_deleted_at` ON `telegram_users` (`deleted_at`);

CREATE TABLE `configs`
(
    `id`         integer,
    `created_at` datetime,
    `updated_at` datetime,
    `deleted_at` datetime,
    `key`        text NOT NULL,
    `value`      text NOT NULL,
    PRIMARY KEY (`id`)
);
CREATE UNIQUE INDEX `idx_configs_key` ON `configs` (`key`) WHERE `deleted_at` is null;
CREATE INDEX `idx_configs_deleted_at` ON `configs` (`deleted_at`);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE `telegram_users`;
DROP TABLE `configs`;
-- +goose StatementEnd
