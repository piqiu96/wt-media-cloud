package service

// Test-only seam for deterministic service tests.
func NewService(store Store, options ...Option) *Service { return newService(store, options...) }
