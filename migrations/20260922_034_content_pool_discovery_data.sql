ALTER TABLE source_contents
    ADD COLUMN view_count BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER favorite_count,
    ADD COLUMN comment_count BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER view_count,
    ADD COLUMN share_count BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER comment_count,
    ADD COLUMN author_sec_uid VARCHAR(255) NULL AFTER author_id,
    ADD COLUMN author_uid VARCHAR(255) NULL AFTER author_sec_uid,
    ADD COLUMN author_home_url TEXT NULL AFTER author_uid;
