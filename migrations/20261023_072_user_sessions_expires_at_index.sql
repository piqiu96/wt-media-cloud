-- 过期会话清理按 expires_at 扫描；已有索引都以 user_id 打头，用不上。
ALTER TABLE user_sessions
    ADD KEY idx_user_sessions_expires_at (expires_at);
