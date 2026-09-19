package service

// Test-only seams keep behavior tests independent of the production database
// registry while production callers use package-level operations.
func NewService(store Store, options ...Option) *Service { return newService(store, options...) }
