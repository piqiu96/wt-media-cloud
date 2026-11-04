-- Runtime and lease DTOs use category_name since the category model replaced
-- games. Rename the persisted 043 column without losing historical values.
-- The 044 published_at column and existing transfer rows remain unchanged.
ALTER TABLE file_transfer_tasks RENAME COLUMN game_name TO category_name;
