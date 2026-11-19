-- Publishing owns this saved input reference. A stored generated foreign key
-- makes task creation and Profile deletion atomic without a reverse callback
-- or an extra pessimistic lock in ordinary foundation mutations.
ALTER TABLE publication_task
  ADD COLUMN browser_profile_id BIGINT UNSIGNED GENERATED ALWAYS AS
    (CAST(JSON_UNQUOTE(JSON_EXTRACT(resource_snapshot_json, '$.browser_profile_id')) AS UNSIGNED)) STORED,
  ADD KEY idx_publication_task_profile (browser_profile_id),
  ADD CONSTRAINT fk_publication_task_profile FOREIGN KEY (browser_profile_id)
    REFERENCES browser_profiles(id) ON DELETE RESTRICT;
