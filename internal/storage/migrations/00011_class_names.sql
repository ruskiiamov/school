-- Имя класса — одно слово без «класс»: интерфейс дописывает «класс» сам
-- (D-115). «7 класс» → «7», «5 А» → «5А». Класс не переименовывается,
-- если новое имя совпало бы с другим классом того же года — такой админ
-- поправит руками.

-- +goose Up
CREATE TEMP TABLE class_names AS
SELECT id, year, name AS old_name,
       REPLACE(TRIM(CASE
           WHEN SUBSTR(name, -5) IN ('класс', 'Класс', 'КЛАСС') THEN SUBSTR(name, 1, LENGTH(name) - 5)
           ELSE name
       END), ' ', '') AS new_name
FROM classes;

UPDATE classes
SET name = (SELECT new_name FROM class_names WHERE class_names.id = classes.id)
WHERE id IN (
    SELECT a.id FROM class_names a
    WHERE a.new_name <> '' AND a.new_name <> a.old_name
      AND NOT EXISTS (
          SELECT 1 FROM class_names b
          WHERE b.year = a.year AND b.id <> a.id AND (b.new_name = a.new_name OR b.old_name = a.new_name)
      )
);

DROP TABLE class_names;

-- +goose Down
SELECT 1;
