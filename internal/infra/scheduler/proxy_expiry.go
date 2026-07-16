package scheduler

import (
	"database/sql"
	"log"
	"time"
)

// StartProxyExpiryChecker runs a periodic check for proxies nearing expiry.
func StartProxyExpiryChecker(db *sql.DB, interval time.Duration) {
	if db == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		// Run once on startup
		checkProxyExpiry(db)
		for range ticker.C {
			checkProxyExpiry(db)
		}
	}()
}

func checkProxyExpiry(db *sql.DB) {
	now := time.Now().UTC()
	sevenDays := now.Add(7 * 24 * time.Hour)
	threeDays := now.Add(3 * 24 * time.Hour)
	oneDay := now.Add(24 * time.Hour)

	// Find proxies expiring within thresholds but not yet notified
	rows, err := db.Query(
		`SELECT id, host, port, expires_at FROM proxy_configs
		 WHERE business_status = 'active' AND expires_at IS NOT NULL
		 AND expires_at <= ? AND expires_at > ?
		 AND (last_check_result NOT LIKE 'expiring_%' OR last_check_result IS NULL)`,
		sevenDays, now,
	)
	if err != nil {
		log.Printf("[scheduler] proxy expiry check query error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, host, port string
		var expiresAt time.Time
		if err := rows.Scan(&id, &host, &port, &expiresAt); err != nil {
			continue
		}
		var label string
		switch {
		case expiresAt.Before(oneDay):
			label = "expiring_today"
		case expiresAt.Before(threeDays):
			label = "expiring_3days"
		case expiresAt.Before(sevenDays):
			label = "expiring_7days"
		default:
			continue
		}
		expired := expiresAt.Before(now)
		if expired {
			label = "expired"
		}
		db.Exec(`UPDATE proxy_configs SET last_check_result = ? WHERE id = ?`, label, id)
		log.Printf("[scheduler] proxy %s (%s:%s) %s", id, host, port, label)
	}
}
