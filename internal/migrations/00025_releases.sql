-- +goose Up
-- The releases that migrated the database (docs/operations.md#upgrades):
-- migrate up refuses to skip a required release or to go back, and serve
-- and worker refuse to run too far behind.
CREATE TABLE horizon_releases (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    version     text NOT NULL,
    commit      text NOT NULL DEFAULT '',
    migrated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE horizon_releases;
