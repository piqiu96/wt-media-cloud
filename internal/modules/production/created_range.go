package production

import (
	"fmt"
	"time"
)

func parseComposeCreatedRange(fromRaw, beforeRaw string) (time.Time, time.Time, error) {
	var from, before time.Time
	var err error
	if fromRaw != "" {
		from, err = time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid created_from: %w", err)
		}
	}
	if beforeRaw != "" {
		before, err = time.Parse(time.RFC3339, beforeRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid created_before: %w", err)
		}
	}
	if !from.IsZero() && !before.IsZero() && !from.Before(before) {
		return time.Time{}, time.Time{}, fmt.Errorf("created_from must precede created_before")
	}
	return from, before, nil
}
