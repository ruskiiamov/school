-- +goose Up
ALTER TABLE users ADD COLUMN last_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN first_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN middle_name TEXT NOT NULL DEFAULT '';

UPDATE users SET
    last_name = CASE WHEN instr(trim(full_name), ' ') = 0 THEN trim(full_name)
        ELSE substr(trim(full_name), 1, instr(trim(full_name), ' ') - 1) END,
    first_name = CASE WHEN instr(trim(full_name), ' ') = 0 THEN ''
        ELSE trim(substr(trim(full_name), instr(trim(full_name), ' ') + 1)) END;

UPDATE users SET
    middle_name = CASE WHEN instr(first_name, ' ') = 0 THEN ''
        ELSE trim(substr(first_name, instr(first_name, ' ') + 1)) END,
    first_name = CASE WHEN instr(first_name, ' ') = 0 THEN first_name
        ELSE substr(first_name, 1, instr(first_name, ' ') - 1) END;

ALTER TABLE users DROP COLUMN full_name;

-- +goose Down
ALTER TABLE users ADD COLUMN full_name TEXT NOT NULL DEFAULT '';

UPDATE users SET full_name = trim(last_name || ' ' || first_name || ' ' || middle_name);

ALTER TABLE users DROP COLUMN middle_name;
ALTER TABLE users DROP COLUMN first_name;
ALTER TABLE users DROP COLUMN last_name;
