-- M2-B Task 8: media-account-to-game is a canonical many-to-many relation.
-- This migration preserves every account, tag, Cookie, check result and Profile
-- binding. It rewrites only the game association that previously lived on the
-- media_accounts base table.

CREATE TABLE media_account_games (
    media_account_id BIGINT UNSIGNED NOT NULL,
    game_id VARCHAR(32) NOT NULL,
    PRIMARY KEY (media_account_id, game_id),
    KEY idx_media_account_games_game_account (game_id, media_account_id),
    CONSTRAINT fk_media_account_games_account FOREIGN KEY (media_account_id) REFERENCES media_accounts (id),
    CONSTRAINT fk_media_account_games_game FOREIGN KEY (game_id) REFERENCES operation_games (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

DELETE FROM media_account_games;

INSERT INTO media_account_games (media_account_id, game_id)
SELECT id, game_id
FROM media_accounts
WHERE game_id IS NOT NULL AND game_id <> '';

ALTER TABLE media_accounts DROP INDEX idx_media_accounts_user_game;
ALTER TABLE media_accounts DROP COLUMN game_id;
