package repository

import "strings"

// likeContains wraps a user-typed term for a `LIKE ? ESCAPE '!'` clause. The
// escape character itself has to be escaped first, or a trailing `!` would
// swallow the wildcard added on either side.
func likeContains(value string) string {
	return "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(value) + "%"
}
