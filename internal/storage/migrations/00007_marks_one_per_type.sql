-- +goose Up
CREATE UNIQUE INDEX idx_marks_lesson_student_type ON marks (lesson_id, student_id, work_type_id);

ALTER TABLE marks DROP COLUMN label;

-- +goose Down
ALTER TABLE marks ADD COLUMN label TEXT NOT NULL DEFAULT '';

DROP INDEX idx_marks_lesson_student_type;
