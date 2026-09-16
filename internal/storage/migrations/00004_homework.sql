-- +goose Up
CREATE TABLE homework (
    lesson_id  INTEGER PRIMARY KEY,
    text       TEXT NOT NULL DEFAULT '',
    due_date   TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE homework_files (
    id           TEXT PRIMARY KEY,
    lesson_id    INTEGER NOT NULL,
    name         TEXT NOT NULL,
    size         INTEGER NOT NULL,
    content_type TEXT NOT NULL,
    created_at   INTEGER NOT NULL
);

CREATE INDEX idx_homework_files_lesson_id ON homework_files (lesson_id);

-- +goose Down
DROP TABLE homework_files;
DROP TABLE homework;
