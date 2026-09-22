-- Add a user nickname for operator name display on audit fields.
ALTER TABLE users ADD COLUMN nickname VARCHAR(64) NULL AFTER username;
UPDATE users SET nickname = username WHERE nickname IS NULL;
