-- Backfill historical rows without a game with the default "其他" game.
UPDATE discovery_strategies SET game_id = 'other' WHERE game_id IS NULL;
UPDATE source_contents SET game_id = 'other' WHERE game_id IS NULL;
