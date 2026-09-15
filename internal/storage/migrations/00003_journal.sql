-- +goose Up
CREATE TABLE substitutions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    class_id   INTEGER NOT NULL,
    subject_id INTEGER NOT NULL,
    teacher_id INTEGER NOT NULL,
    start_date TEXT NOT NULL,
    end_date   TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX idx_substitutions_teacher_id ON substitutions (teacher_id);
CREATE INDEX idx_substitutions_class_subject ON substitutions (class_id, subject_id);

CREATE TABLE lessons (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    class_id   INTEGER NOT NULL,
    subject_id INTEGER NOT NULL,
    teacher_id INTEGER NOT NULL,
    date       TEXT NOT NULL,
    topic      TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (class_id, subject_id, date)
);

CREATE INDEX idx_lessons_teacher_id ON lessons (teacher_id);

CREATE TABLE marks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    lesson_id    INTEGER NOT NULL,
    student_id   INTEGER NOT NULL,
    work_type_id INTEGER NOT NULL,
    value        INTEGER NOT NULL,
    label        TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE INDEX idx_marks_lesson_id ON marks (lesson_id);
CREATE INDEX idx_marks_student_id ON marks (student_id);

CREATE TABLE lesson_students (
    lesson_id  INTEGER NOT NULL,
    student_id INTEGER NOT NULL,
    absent     INTEGER NOT NULL DEFAULT 0,
    comment    TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (lesson_id, student_id)
);

CREATE INDEX idx_lesson_students_student_id ON lesson_students (student_id);

-- +goose Down
DROP TABLE lesson_students;
DROP TABLE marks;
DROP TABLE lessons;
DROP TABLE substitutions;
