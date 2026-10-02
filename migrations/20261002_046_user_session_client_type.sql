-- Session-layer client_type decoupling (CHG-20261002-074 stage 1).
-- client_type is inferred server-side from the Origin header (Tauri WebView
-- origin -> desktop, anything else -> web) and persisted on the session row, so
-- desktop and web sessions coexist and login replacement only kicks same-type
-- sessions. Legacy sessions predate the feature and cannot be distinguished, so
-- they are backfilled as web; a desktop session is recreated with its real type
-- on the next login.
ALTER TABLE user_sessions
  ADD COLUMN client_type VARCHAR(16) NOT NULL DEFAULT 'web' AFTER token_hash;
