-- Associate discovery strategies and source contents with an operation game.
ALTER TABLE discovery_strategies
  ADD COLUMN game_id VARCHAR(32) NULL AFTER team_id,
  ADD KEY idx_discovery_strategies_game (game_id),
  ADD CONSTRAINT fk_discovery_strategies_game FOREIGN KEY (game_id) REFERENCES operation_games (id) ON DELETE SET NULL;

ALTER TABLE source_contents
  ADD COLUMN game_id VARCHAR(32) NULL AFTER team_id,
  ADD KEY idx_source_contents_game (game_id),
  ADD CONSTRAINT fk_source_contents_game FOREIGN KEY (game_id) REFERENCES operation_games (id) ON DELETE SET NULL;
