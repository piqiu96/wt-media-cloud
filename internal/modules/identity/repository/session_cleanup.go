package repository

import (
	"context"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"gorm.io/gorm"
)

// DeleteExpiredSessionsBefore 删除 cutoff 之前过期的会话行。
// 不按 invalidated_at 过滤：未过期的失效行要留到过期，旧 Desktop 才能继续收到
// AUTH_DESKTOP_REPLACED，而不是被当成不存在的会话。
func DeleteExpiredSessionsBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return deleteExpiredSessionsBefore(ctx, database.DB(), cutoff, limit)
}

func deleteExpiredSessionsBefore(ctx context.Context, db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
	return execSQL(db.WithContext(ctx), `DELETE FROM user_sessions WHERE expires_at < ? LIMIT ?`, cutoff, limit)
}
