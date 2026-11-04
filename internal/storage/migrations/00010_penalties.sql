-- +goose Up
CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

ALTER TABLE work_types ADD COLUMN penalized INTEGER NOT NULL DEFAULT 0;

CREATE TABLE penalties (
    id           INTEGER PRIMARY KEY,
    lesson_id    INTEGER NOT NULL,
    student_id   INTEGER NOT NULL,
    work_type_id INTEGER NOT NULL,
    done_percent INTEGER NOT NULL,
    points       INTEGER NOT NULL,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE UNIQUE INDEX idx_penalties_cell ON penalties (lesson_id, student_id, work_type_id);
CREATE INDEX idx_penalties_student_id ON penalties (student_id);

CREATE TABLE penalty_workoffs (
    id          INTEGER PRIMARY KEY,
    student_id  INTEGER NOT NULL,
    date        TEXT NOT NULL,
    points      INTEGER NOT NULL,
    comment     TEXT NOT NULL DEFAULT '',
    recorded_by INTEGER NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE INDEX idx_penalty_workoffs_student_id ON penalty_workoffs (student_id);

-- +goose Down
DROP TABLE penalty_workoffs;
DROP TABLE penalties;
ALTER TABLE work_types DROP COLUMN penalized;
DROP TABLE settings;
