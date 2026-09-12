package proxy

import (
	"database/sql"
	"fmt"
	"strings"
)

const proxyColumns = `id, source_type, proxy_protocol, host, port, username, password, region, supplier, extract_url, expires_at, business_status, max_profile_count, last_check_at, last_check_result, observed_exit_ip, remark, created_at, updated_at`

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) Create(p ProxyConfig) error {
	_, err := s.db.Exec(
		`INSERT INTO proxy_configs (`+proxyColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.SourceType, p.ProxyProtocol, p.Host, p.Port,
		nullIfEmpty(p.Username), nullIfEmpty(p.Password),
		nullIfEmpty(p.Region), nullIfEmpty(p.Supplier), nullIfEmpty(p.ExtractURL),
		p.ExpiresAt, p.BusinessStatus, p.MaxProfileCount,
		p.LastCheckAt, nullIfEmpty(p.LastCheckResult),
		nullIfEmpty(p.ObservedExitIP), nullIfEmpty(p.Remark),
		p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *MySQLStore) FindByID(id string) (ProxyConfig, bool, error) {
	row := s.db.QueryRow(`SELECT `+proxyColumns+` FROM proxy_configs WHERE id = ?`, id)
	p, err := scanProxy(row)
	if err == sql.ErrNoRows {
		return ProxyConfig{}, false, nil
	}
	if err != nil {
		return ProxyConfig{}, false, err
	}
	return p, true, nil
}

func (s *MySQLStore) List(filter ProxyFilter) ([]ProxyConfig, error) {
	var conditions []string
	var args []interface{}

	if filter.BusinessStatus != "" {
		conditions = append(conditions, "business_status = ?")
		args = append(args, filter.BusinessStatus)
	}
	if filter.Supplier != "" {
		conditions = append(conditions, "supplier = ?")
		args = append(args, filter.Supplier)
	}
	if filter.Region != "" {
		conditions = append(conditions, "region = ?")
		args = append(args, filter.Region)
	}
	if filter.Search != "" {
		conditions = append(conditions, "(host LIKE ? OR remark LIKE ? OR supplier LIKE ?)")
		s := "%" + filter.Search + "%"
		args = append(args, s, s, s)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf("SELECT %s FROM proxy_configs%s ORDER BY created_at DESC LIMIT ? OFFSET ?", proxyColumns, where)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Keep collection endpoints JSON-array shaped even when the database is empty.
	results := make([]ProxyConfig, 0)
	for rows.Next() {
		p, err := scanProxy(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

func (s *MySQLStore) Update(p ProxyConfig) error {
	_, err := s.db.Exec(
		`UPDATE proxy_configs SET source_type=?, proxy_protocol=?, host=?, port=?, username=?, password=?, region=?, supplier=?, extract_url=?, expires_at=?, business_status=?, max_profile_count=?, last_check_at=?, last_check_result=?, observed_exit_ip=?, remark=?, updated_at=? WHERE id=?`,
		p.SourceType, p.ProxyProtocol, p.Host, p.Port,
		nullIfEmpty(p.Username), nullIfEmpty(p.Password),
		nullIfEmpty(p.Region), nullIfEmpty(p.Supplier), nullIfEmpty(p.ExtractURL),
		p.ExpiresAt, p.BusinessStatus, p.MaxProfileCount,
		p.LastCheckAt, nullIfEmpty(p.LastCheckResult),
		nullIfEmpty(p.ObservedExitIP), nullIfEmpty(p.Remark),
		p.UpdatedAt, p.ID,
	)
	return err
}

func (s *MySQLStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM proxy_configs WHERE id = ?`, id)
	return err
}

// --- Scanner ---

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanProxy(row scannable) (ProxyConfig, error) {
	var p ProxyConfig
	var username, password, region, supplier, extractURL, lastCheckResult, observedExitIP, remark sql.NullString
	var expiresAt, lastCheckAt sql.NullTime

	err := row.Scan(
		&p.ID, &p.SourceType, &p.ProxyProtocol, &p.Host, &p.Port,
		&username, &password, &region, &supplier,
		&extractURL, &expiresAt, &p.BusinessStatus, &p.MaxProfileCount,
		&lastCheckAt, &lastCheckResult, &observedExitIP, &remark,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return ProxyConfig{}, err
	}

	p.Username = username.String
	p.Password = password.String
	p.Region = region.String
	p.Supplier = supplier.String
	p.ExtractURL = extractURL.String
	if expiresAt.Valid {
		p.ExpiresAt = &expiresAt.Time
	}
	if lastCheckAt.Valid {
		p.LastCheckAt = &lastCheckAt.Time
	}
	p.LastCheckResult = lastCheckResult.String
	p.ObservedExitIP = observedExitIP.String
	p.Remark = remark.String

	return p, nil
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
