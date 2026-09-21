package repository

// ProxyFilter is the persistence projection of proxy query criteria.
type ProxyFilter struct {
	Platform       string
	Supplier       string
	BusinessStatus string
	Region         string
	Search         string
	Limit          int
	Offset         int
}
