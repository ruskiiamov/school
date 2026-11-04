-- +goose Up
ALTER TABLE work_types ADD COLUMN teacher_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE work_types ADD COLUMN subject_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE work_types ADD COLUMN source_id INTEGER NOT NULL DEFAULT 0;

INSERT INTO work_types (name, sort_order, active, created_at, updated_at, teacher_id, subject_id, source_id)
SELECT w.name, w.sort_order, w.active, w.created_at, w.updated_at, owners.teacher_id, owners.subject_id, w.id
FROM work_types w
CROSS JOIN (
    SELECT teacher_id, subject_id FROM teaching_assignments
    UNION
    SELECT teacher_id, subject_id FROM substitutions
    UNION
    SELECT teacher_id, subject_id FROM lessons
) owners
WHERE w.teacher_id = 0;

DELETE FROM marks WHERE lesson_id NOT IN (SELECT id FROM lessons);

UPDATE marks SET work_type_id = (
    SELECT copy.id
    FROM work_types copy
    JOIN lessons l ON l.id = marks.lesson_id
    WHERE copy.source_id = marks.work_type_id AND copy.teacher_id = l.teacher_id AND copy.subject_id = l.subject_id
);

DELETE FROM work_types WHERE teacher_id = 0;

ALTER TABLE work_types DROP COLUMN source_id;

CREATE INDEX idx_work_types_owner ON work_types (teacher_id, subject_id);

-- +goose Down
DROP INDEX idx_work_types_owner;

INSERT INTO work_types (name, sort_order, active, created_at, updated_at, teacher_id, subject_id)
SELECT name, MIN(sort_order), MAX(active), MIN(created_at), MAX(updated_at), 0, 0
FROM work_types
WHERE teacher_id != 0
GROUP BY name;

UPDATE marks SET work_type_id = (
    SELECT shared.id
    FROM work_types shared
    JOIN work_types own ON own.name = shared.name
    WHERE shared.teacher_id = 0 AND own.id = marks.work_type_id
);

DELETE FROM work_types WHERE teacher_id != 0;

ALTER TABLE work_types DROP COLUMN subject_id;
ALTER TABLE work_types DROP COLUMN teacher_id;
