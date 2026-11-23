ALTER TABLE operations_files ADD COLUMN metadata_json JSON NULL;

CREATE TABLE image_asset (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
 team_id BIGINT UNSIGNED NOT NULL,
 file_id BIGINT NOT NULL,
 name VARCHAR(160) NOT NULL,
 allow_as_cover TINYINT(1) NOT NULL DEFAULT 1,
 allow_as_background TINYINT(1) NOT NULL DEFAULT 0,
 category_ids_json JSON NOT NULL,
 enabled TINYINT(1) NOT NULL DEFAULT 1,
 revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
 created_by BIGINT UNSIGNED NOT NULL,
 updated_by BIGINT UNSIGNED NOT NULL,
 created_at DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 PRIMARY KEY (id),
 UNIQUE KEY uq_image_asset_file (file_id),
 KEY idx_image_asset_team (team_id,enabled,created_at,id),
 CONSTRAINT fk_image_asset_file FOREIGN KEY (file_id) REFERENCES operations_files(id),
 CONSTRAINT fk_image_asset_team FOREIGN KEY (team_id) REFERENCES operation_teams(id),
 CONSTRAINT chk_image_asset_usage CHECK (allow_as_cover=1 OR allow_as_background=1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
