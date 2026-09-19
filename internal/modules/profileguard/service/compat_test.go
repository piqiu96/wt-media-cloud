package service

// Test-only seam for deterministic service tests.
func NewService(store Store, nodes NodeAuthenticator, options ...Option) *Service {
	return newService(store, nodes, options...)
}
