-- +goose Up
ALTER TABLE users ADD COLUMN active INTEGER NOT NULL DEFAULT 1;

CREATE TABLE subjects (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    active     INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE work_types (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    active     INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE classes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    year       INTEGER NOT NULL,
    name       TEXT NOT NULL,
    graduating INTEGER NOT NULL DEFAULT 0,
    active     INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (year, name)
);

CREATE TABLE class_students (
    class_id   INTEGER NOT NULL,
    student_id INTEGER NOT NULL,
    PRIMARY KEY (class_id, student_id)
);

CREATE INDEX idx_class_students_student_id ON class_students (student_id);

CREATE TABLE parent_children (
    parent_id  INTEGER NOT NULL,
    student_id INTEGER NOT NULL,
    PRIMARY KEY (parent_id, student_id)
);

CREATE INDEX idx_parent_children_student_id ON parent_children (student_id);

CREATE TABLE teaching_assignments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    teacher_id INTEGER NOT NULL,
    subject_id INTEGER NOT NULL,
    class_id   INTEGER NOT NULL,
    UNIQUE (subject_id, class_id)
);

CREATE INDEX idx_teaching_assignments_teacher_id ON teaching_assignments (teacher_id);

INSERT INTO work_types (name, sort_order, active, created_at, updated_at) VALUES
    ('Ответ на уроке', 10, 1, strftime('%s', 'now') * 1000, strftime('%s', 'now') * 1000),
    ('Классная работа', 20, 1, strftime('%s', 'now') * 1000, strftime('%s', 'now') * 1000),
    ('Самостоятельная работа', 30, 1, strftime('%s', 'now') * 1000, strftime('%s', 'now') * 1000),
    ('Контрольная работа', 40, 1, strftime('%s', 'now') * 1000, strftime('%s', 'now') * 1000),
    ('Домашняя работа (письменная)', 50, 1, strftime('%s', 'now') * 1000, strftime('%s', 'now') * 1000),
    ('Домашняя работа (устная)', 60, 1, strftime('%s', 'now') * 1000, strftime('%s', 'now') * 1000);

-- +goose Down
DROP TABLE teaching_assignments;
DROP TABLE parent_children;
DROP TABLE class_students;
DROP TABLE classes;
DROP TABLE work_types;
DROP TABLE subjects;
ALTER TABLE users DROP COLUMN active;
