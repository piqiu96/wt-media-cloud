-- Administrator verification is a human fact linked to the uncertain
-- execution attempt. It never changes the worker's lease safety rules.
CREATE TABLE compose_uncertain_reviews (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    task_id BIGINT UNSIGNED NOT NULL,
    attempt INT UNSIGNED NOT NULL,
    actor_user_id BIGINT UNSIGNED NOT NULL,
    evidence VARCHAR(2000) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_compose_uncertain_reviews_task_attempt (task_id, attempt),
    KEY idx_compose_uncertain_reviews_task (task_id, id),
    CONSTRAINT fk_compose_uncertain_reviews_task FOREIGN KEY (task_id) REFERENCES compose_tasks(id),
    CONSTRAINT fk_compose_uncertain_reviews_actor FOREIGN KEY (actor_user_id) REFERENCES users(id),
    CONSTRAINT chk_compose_uncertain_reviews_evidence CHECK (CHAR_LENGTH(evidence) BETWEEN 20 AND 2000)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
