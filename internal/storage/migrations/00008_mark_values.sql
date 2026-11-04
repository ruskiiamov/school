-- +goose Up
CREATE TABLE marks_new (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    lesson_id    INTEGER NOT NULL,
    student_id   INTEGER NOT NULL,
    work_type_id INTEGER NOT NULL,
    value        TEXT NOT NULL,
    comment      TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

INSERT INTO marks_new (id, lesson_id, student_id, work_type_id, value, created_at, updated_at)
SELECT id, lesson_id, student_id, work_type_id, CAST(value AS TEXT), created_at, updated_at FROM marks;

DROP TABLE marks;

ALTER TABLE marks_new RENAME TO marks;

CREATE INDEX idx_marks_lesson_id ON marks (lesson_id);
CREATE INDEX idx_marks_student_id ON marks (student_id);
CREATE UNIQUE INDEX idx_marks_lesson_student_type ON marks (lesson_id, student_id, work_type_id);

-- +goose Down
CREATE TABLE marks_old (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    lesson_id    INTEGER NOT NULL,
    student_id   INTEGER NOT NULL,
    work_type_id INTEGER NOT NULL,
    value        INTEGER NOT NULL,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

INSERT INTO marks_old (id, lesson_id, student_id, work_type_id, value, created_at, updated_at)
SELECT id, lesson_id, student_id, work_type_id, CAST(value AS INTEGER), created_at, updated_at FROM marks;

DROP TABLE marks;

ALTER TABLE marks_old RENAME TO marks;

CREATE INDEX idx_marks_lesson_id ON marks (lesson_id);
CREATE INDEX idx_marks_student_id ON marks (student_id);
CREATE UNIQUE INDEX idx_marks_lesson_student_type ON marks (lesson_id, student_id, work_type_id);
