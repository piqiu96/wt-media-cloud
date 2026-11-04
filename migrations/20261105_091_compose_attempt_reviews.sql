-- Abort on a corrupt/orphan review rather than inventing an execution log.
ALTER TABLE compose_uncertain_reviews
  ADD CONSTRAINT fk_compose_review_attempt_migration
    FOREIGN KEY(task_id,attempt) REFERENCES compose_task_attempts(task_id,attempt);

-- A review has the same identity and lifetime as the uncertain attempt.
ALTER TABLE compose_task_attempts
  ADD COLUMN uncertain_review_actor_id BIGINT UNSIGNED NULL,
  ADD COLUMN uncertain_review_evidence TEXT NULL,
  ADD COLUMN uncertain_reviewed_at DATETIME(6) NULL,
  ADD CONSTRAINT fk_compose_attempt_review_actor FOREIGN KEY(uncertain_review_actor_id) REFERENCES users(id),
  ADD CONSTRAINT chk_compose_attempt_review CHECK (
    (uncertain_review_actor_id IS NULL AND uncertain_review_evidence IS NULL AND uncertain_reviewed_at IS NULL)
    OR (uncertain_review_actor_id IS NOT NULL AND uncertain_review_evidence IS NOT NULL
        AND uncertain_reviewed_at IS NOT NULL AND CHAR_LENGTH(uncertain_review_evidence) BETWEEN 20 AND 2000)
  );
UPDATE compose_task_attempts a JOIN compose_uncertain_reviews r
  ON a.task_id=r.task_id AND a.attempt=r.attempt
SET a.uncertain_review_actor_id=r.actor_user_id,
    a.uncertain_review_evidence=r.evidence,
    a.uncertain_reviewed_at=r.created_at;
DROP TABLE compose_uncertain_reviews;
