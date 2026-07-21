package proxy

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
	p.LastCheckResult = result
	p.UpdatedAt = now
	if err := s.store.Update(p); err != nil {
		return ProxyConfig{}, err
	}
	return p, nil
}
