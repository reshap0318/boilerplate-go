-- Soft delete + UNIQUE fix.
--
-- users.email, roles.name and permissions.name were plain UNIQUE columns, but rows
-- are soft-deleted (deleted_at). GORM hides soft-deleted rows, so the service-level
-- "already exists?" check passes, then the INSERT hits the old row's UNIQUE key and
-- fails with a 500. Example: delete user a@x.com, create a@x.com again -> error.
--
-- MySQL has no partial indexes, so the uniqueness is moved to (column, alive) where
-- `alive` is a generated column: 1 for live rows, NULL for soft-deleted rows.
-- NULLs never collide in a UNIQUE index, so any number of deleted rows may share a
-- value while live rows stay unique.

-- +goose Up
ALTER TABLE users
    ADD COLUMN alive TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) VIRTUAL,
    ADD UNIQUE INDEX uq_users_email_alive (email, alive);
ALTER TABLE users DROP INDEX email;

ALTER TABLE roles
    ADD COLUMN alive TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) VIRTUAL,
    ADD UNIQUE INDEX uq_roles_name_alive (name, alive);
ALTER TABLE roles DROP INDEX name;

ALTER TABLE permissions
    ADD COLUMN alive TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) VIRTUAL,
    ADD UNIQUE INDEX uq_permissions_name_alive (name, alive);
ALTER TABLE permissions DROP INDEX name;

-- Notification list/count: WHERE user_id = ? AND deleted_at IS NULL [AND read_at IS NULL]
-- ORDER BY created_at DESC. The composite index serves the filter and the sort; it
-- also starts with user_id, so it replaces the single-column index for the FK.
ALTER TABLE notifications ADD INDEX idx_notifications_user_deleted_created (user_id, deleted_at, created_at);
ALTER TABLE notifications DROP INDEX idx_notifications_user_id;

-- +goose Down
ALTER TABLE notifications ADD INDEX idx_notifications_user_id (user_id);
ALTER TABLE notifications DROP INDEX idx_notifications_user_deleted_created;

ALTER TABLE permissions ADD UNIQUE INDEX name (name);
ALTER TABLE permissions DROP INDEX uq_permissions_name_alive, DROP COLUMN alive;

ALTER TABLE roles ADD UNIQUE INDEX name (name);
ALTER TABLE roles DROP INDEX uq_roles_name_alive, DROP COLUMN alive;

ALTER TABLE users ADD UNIQUE INDEX email (email);
ALTER TABLE users DROP INDEX uq_users_email_alive, DROP COLUMN alive;
