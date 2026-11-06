CREATE TABLE operations_files (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    owner_id BIGINT NOT NULL,
    team_id BIGINT NULL,
    name VARCHAR(160) NOT NULL,
    extension VARCHAR(8) NOT NULL,
    type VARCHAR(16) NOT NULL,
    content_type VARCHAR(80) NOT NULL,
    object_key VARCHAR(1024) NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
    resource_token CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    uploaded_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    UNIQUE KEY uq_operations_files_token (resource_token),
    KEY idx_operations_files_owner (owner_id,created_at),
    KEY idx_operations_files_team (team_id,created_at)
);
