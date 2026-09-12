package proxy

import "strings"

func (s *Service) RecordCheckResult(id, result string) (ProxyConfig, error) {
	p, ok, err := s.store.FindByID(id)
	if err != nil {
		return ProxyConfig{}, err
	}
	if !ok {
		return ProxyConfig{}, ErrNotFound
	}
	now := s.now()
	p.LastCheckAt = &now
	result = strings.TrimSpace(result)
	if result == "reachable" {
		// Local Agent reports the transport fact "reachable". The Cloud proxy
		// lifecycle uses "ok" as its canonical eligible state, so normalize at
		// the owning boundary instead of making every consumer understand both.
		result = "ok"
	}
	p.LastCheckResult = result
	p.UpdatedAt = now
	if err := s.store.Update(p); err != nil {
		return ProxyConfig{}, err
	}
	return p, nil
}
