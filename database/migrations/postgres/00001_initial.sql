-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

CREATE TABLE "configs"
(
    "id"         bigserial,
    "created_at" timestamptz,
    "updated_at" timestamptz,
    "deleted_at" timestamptz DEFAULT NULL,
    "key"        varchar(250) NOT NULL,
    "value"      varchar(250) NOT NULL,
    PRIMARY KEY ("id")
);

CREATE INDEX "idx_configs_deleted_at" ON "configs" ("deleted_at");

CREATE UNIQUE INDEX "idx_configs_key" ON "configs" ("key") WHERE "deleted_at" is null;

CREATE TABLE "telegram_users"
(
    "id"                                   bigint,
    "created_at"                           timestamptz,
    "updated_at"                           timestamptz,
    "deleted_at"                           timestamptz DEFAULT NULL,
    "first_name"                           varchar(250),
    "last_name"                            varchar(250),
    "username"                             varchar(250),
    "language_code"                        varchar(3),
    "is_admin"                             boolean     DEFAULT FALSE,
    "accepted_terms_and_conditions_on"     timestamptz DEFAULT NULL,
    "accepted_latest_terms_and_conditions" boolean     DEFAULT FALSE,
    PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX "idx_telegram_users_username" ON "telegram_users" ("username") WHERE "deleted_at" is null;

CREATE INDEX "idx_telegram_users_deleted_at" ON "telegram_users" ("deleted_at");

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE "telegram_users";
DROP TABLE "configs";
-- +goose StatementEnd
