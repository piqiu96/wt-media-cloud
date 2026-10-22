-- user_sessions becomes the single authentication fact source. Every session
-- gets an absolute expiry, and every invalidation records why it happened so
-- the API can distinguish a replaced Desktop login from a password change,
-- forced logout or ordinary expiry.
--
-- expires_at is added nullable and backfilled from created_at before being made
-- NOT NULL: an ALTER with a non-null default would silently give every legacy
-- session a fresh 30-day life at migration time rather than preserving its age.
ALTER TABLE user_sessions
    ADD COLUMN expires_at DATETIME(6) NULL AFTER created_at,
    ADD COLUMN invalidated_reason VARCHAR(32) NULL AFTER invalidated_at;

UPDATE user_sessions
SET expires_at = DATE_ADD(created_at, INTERVAL 30 DAY)
WHERE expires_at IS NULL;

-- Older login code replaced every session of a client type, but a crash between
-- invalidating and inserting could leave two live Desktop rows. Keep the newest
-- row (id breaks created_at ties) and mark the rest superseded so the login path
-- can rely on at most one active Desktop session.
CREATE TEMPORARY TABLE m2_f_superseded_desktop_sessions (
    id VARCHAR(64) NOT NULL,
    PRIMARY KEY (id)
);

INSERT INTO m2_f_superseded_desktop_sessions (id)
SELECT id
FROM (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC, id DESC) AS rn
    FROM user_sessions
    WHERE client_type = 'desktop'
      AND invalidated_at IS NULL
      AND expires_at > UTC_TIMESTAMP(6)
) ranked
WHERE ranked.rn > 1;

UPDATE user_sessions sessions
JOIN m2_f_superseded_desktop_sessions duplicates ON duplicates.id = sessions.id
SET sessions.invalidated_at = UTC_TIMESTAMP(6),
    sessions.invalidated_reason = 'superseded';

DROP TEMPORARY TABLE m2_f_superseded_desktop_sessions;

-- Fail the migration rather than leaving the login path to fail closed later if
-- the collapse above ever missed a user.
CREATE TEMPORARY TABLE m2_f_session_guard (
    valid TINYINT NOT NULL,
    CONSTRAINT chk_m2_f_session_guard CHECK (valid = 1)
);

INSERT INTO m2_f_session_guard (valid)
SELECT IF(
    (SELECT COUNT(*) FROM (
        SELECT user_id
        FROM user_sessions
        WHERE client_type = 'desktop'
          AND invalidated_at IS NULL
          AND expires_at > UTC_TIMESTAMP(6)
        GROUP BY user_id
        HAVING COUNT(*) > 1
    ) duplicates) = 0,
    1,
    0
);

DROP TEMPORARY TABLE m2_f_session_guard;

ALTER TABLE user_sessions
    MODIFY COLUMN expires_at DATETIME(6) NOT NULL,
    ADD KEY idx_user_sessions_user_client_active (user_id, client_type, invalidated_at, expires_at);
