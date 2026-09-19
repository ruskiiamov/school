-- +goose Up
CREATE TABLE login_devices (
    token_hash   TEXT PRIMARY KEY,
    user_id      INTEGER NOT NULL,
    created_at   INTEGER NOT NULL,
    last_used_at INTEGER NOT NULL
);

CREATE INDEX idx_login_devices_user_id ON login_devices (user_id);

-- +goose Down
DROP TABLE login_devices;
