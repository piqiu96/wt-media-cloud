-- M4-A production facts. Source materialization remains metadata-only; no
-- worker, object-storage connection, local path, or signed URL is introduced.
ALTER TABLE materials ADD COLUMN video_status VARCHAR(32) NOT NULL DEFAULT 'not_downloaded' AFTER source_snapshot;
ALTER TABLE materials ADD COLUMN source_object_key VARCHAR(512) NULL AFTER video_status;
ALTER TABLE materials ADD COLUMN video_size_bytes BIGINT UNSIGNED NULL AFTER source_object_key;
ALTER TABLE materials ADD COLUMN video_sha256 CHAR(64) NULL AFTER video_size_bytes;
ALTER TABLE materials ADD COLUMN video_media_json JSON NULL AFTER video_sha256;
ALTER TABLE materials ADD COLUMN video_error VARCHAR(500) NULL AFTER video_media_json;
ALTER TABLE materials ADD COLUMN video_prepared_at DATETIME(6) NULL AFTER video_error;
ALTER TABLE materials ADD CONSTRAINT chk_materials_video_status CHECK (video_status IN ('not_downloaded', 'downloading', 'ready', 'failed'));

CREATE TABLE material_usages (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    team_id BIGINT UNSIGNED NOT NULL,
    material_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    removed_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_material_usages_user_material (user_id, material_id),
    KEY idx_material_usages_team_status (team_id, status, updated_at),
    KEY idx_material_usages_material (material_id),
    CONSTRAINT fk_material_usages_team FOREIGN KEY (team_id) REFERENCES operation_teams(id),
    CONSTRAINT fk_material_usages_material FOREIGN KEY (material_id) REFERENCES materials(id),
    CONSTRAINT fk_material_usages_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT chk_material_usages_status CHECK (status IN ('active', 'removed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE file_transfer_tasks (
    id CHAR(36) NOT NULL,
    team_id BIGINT UNSIGNED NOT NULL,
    asset_type VARCHAR(32) NOT NULL,
    asset_id BIGINT UNSIGNED NOT NULL,
    purpose VARCHAR(32) NOT NULL,
    execution_scope VARCHAR(32) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    requested_by BIGINT UNSIGNED NOT NULL,
    assigned_node_id VARCHAR(128) NULL,
    claimed_by_node_id VARCHAR(128) NULL,
    dependency_task_id CHAR(36) NULL,
    dedupe_key CHAR(64) NOT NULL,
    total_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
    transferred_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
    speed_bytes_per_sec BIGINT UNSIGNED NOT NULL DEFAULT 0,
    eta_seconds BIGINT UNSIGNED NULL,
    attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
    max_attempts INT UNSIGNED NOT NULL DEFAULT 3,
    lease_expires_at DATETIME(6) NULL,
    heartbeat_at DATETIME(6) NULL,
    started_at DATETIME(6) NULL,
    finished_at DATETIME(6) NULL,
    error_code VARCHAR(128) NULL,
    error_message VARCHAR(500) NULL,
    integrity_sha256 CHAR(64) NULL,
    integrity_bytes BIGINT UNSIGNED NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_file_transfer_tasks_dedupe (dedupe_key),
    KEY idx_file_transfer_tasks_queue (execution_scope, status, created_at),
    KEY idx_file_transfer_tasks_assigned_node (assigned_node_id, status, created_at),
    KEY idx_file_transfer_tasks_material (asset_id, purpose, status),
    CONSTRAINT fk_file_transfer_tasks_team FOREIGN KEY (team_id) REFERENCES operation_teams(id),
    CONSTRAINT fk_file_transfer_tasks_requested_by FOREIGN KEY (requested_by) REFERENCES users(id),
    CONSTRAINT fk_file_transfer_tasks_dependency FOREIGN KEY (dependency_task_id) REFERENCES file_transfer_tasks(id),
    CONSTRAINT chk_file_transfer_tasks_asset_type CHECK (asset_type IN ('material', 'composite_output')),
    CONSTRAINT chk_file_transfer_tasks_purpose CHECK (purpose IN ('compose_input_prepare', 'user_download')),
    CONSTRAINT chk_file_transfer_tasks_scope CHECK (execution_scope IN ('cloud', 'local_agent')),
    CONSTRAINT chk_file_transfer_tasks_status CHECK (status IN ('pending', 'running', 'success', 'failed', 'cancelled')),
    CONSTRAINT chk_file_transfer_tasks_counts CHECK (transferred_bytes <= total_bytes OR total_bytes = 0),
    CONSTRAINT chk_file_transfer_tasks_scope_purpose CHECK ((purpose = 'compose_input_prepare' AND execution_scope = 'cloud') OR (purpose = 'user_download' AND execution_scope = 'local_agent'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
