package service

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"

	"github.com/wt-media/wt-media-cloud/internal/modules/files/repository"
	"gorm.io/gorm"
)

// LockJSONReferences protects existing saved production URLs without a second
// reference registry. Call in the same transaction that writes the JSON.
func LockJSONReferences(tx *gorm.DB, documents ...json.RawMessage) error {
	tokens, err := referenceTokens(documents...)
	if err != nil {
		return err
	}
	ids := make([]int64, 0, len(tokens))
	for _, token := range tokens {
		file, err := (repository.Store{DB: tx}).FindToken(token)
		if err != nil {
			return err
		}
		ids = append(ids, file.ID)
	}
	return LockAvailable(tx, ids)
}
func referenceTokens(documents ...json.RawMessage) ([]string, error) {
	found := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case string:
			u, err := url.Parse(v)
			if err != nil {
				return
			}
			const prefix = "/api/v1/file-resources/"
			if strings.HasPrefix(u.Path, prefix) {
				token := strings.TrimPrefix(u.Path, prefix)
				if token != "" && !strings.Contains(token, "/") {
					found[token] = true
				}
			}
		case []any:
			for _, item := range v {
				visit(item)
			}
		case map[string]any:
			for _, item := range v {
				visit(item)
			}
		}
	}
	for _, raw := range documents {
		if len(raw) == 0 {
			continue
		}
		var value any
		if json.Unmarshal(raw, &value) != nil {
			return nil, ErrInvalid
		}
		visit(value)
	}
	tokens := make([]string, 0, len(found))
	for token := range found {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return tokens, nil
}
