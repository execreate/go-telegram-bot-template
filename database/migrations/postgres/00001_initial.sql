-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

CREATE TABLE "telegram_users"
(
    "id"                                   bigint,
    "created_at"                           timestamptz,
    "updated_at"                           timestamptz,
    "deleted_at"                           timestamptz,
    "first_name"                           varchar(250),
    "last_name"                            varchar(250),
    "username"                             varchar(250),
    "language_code"                        varchar(3),
    "is_admin"                             boolean,
    "accepted_terms_and_conditions_on"     timestamptz,
    "accepted_latest_terms_and_conditions" boolean,
    PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX IF NOT EXISTS "idx_telegram_users_username" ON "telegram_users" ("username");
CREATE INDEX IF NOT EXISTS "idx_telegram_users_deleted_at" ON "telegram_users" ("deleted_at");

CREATE TABLE "configs"
(
    "id"         bigserial,
    "created_at" timestamptz,
    "updated_at" timestamptz,
    "deleted_at" timestamptz,
    "key"        varchar(250) NOT NULL,
    "value"      varchar(250) NOT NULL,
    PRIMARY KEY ("id")
);
CREATE INDEX IF NOT EXISTS "idx_configs_deleted_at" ON "configs" ("deleted_at");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_configs_key" ON "configs" ("key");
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE "telegram_users";
DROP TABLE "configs";
-- +goose StatementEnd
