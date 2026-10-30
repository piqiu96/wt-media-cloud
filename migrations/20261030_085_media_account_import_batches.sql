CREATE TABLE IF NOT EXISTS media_account_import_batches (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
 user_id BIGINT UNSIGNED NOT NULL,
 request_key VARCHAR(64) NOT NULL,
 platform VARCHAR(32) NOT NULL,
 content_json JSON NOT NULL,
 revision BIGINT UNSIGNED NOT NULL DEFAULT 0,
 created_at DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 PRIMARY KEY (id),
 UNIQUE KEY uk_media_import_request (user_id, request_key),
 KEY idx_media_import_user_created (user_id, created_at),
 CONSTRAINT fk_media_import_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
