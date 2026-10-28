package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

const proxyColumns = `id, team_id, name, source, protocol, host, port, username, password, region, supplier, tags, extract_url, expires_at, enabled, max_profiles, last_check_at, last_check_result, last_check_message, remark, created_at, updated_at`

// ConnectionKey 是团队内查重用的规范化连接标识；带上所属记录，预览才能指出
// 与哪一条冲突。
type ConnectionKey struct {
	ID       string
	Protocol string
	Host     string
	Port     int
}

func Create(p model.Proxy) error {
	return create(database.DB(), p)
}

func create(db *gorm.DB, p model.Proxy) error {
	err := execSQL(db,
		`INSERT INTO proxies (`+proxyColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.TeamID, p.Name, p.Source, p.Protocol, p.Host, p.Port,
		nullIfEmpty(p.Username), nullIfEmpty(p.Password),
		nullIfEmpty(p.Region), nullIfEmpty(p.Supplier), nullIfEmpty(joinTags(p.Tags)), nullIfEmpty(p.ExtractURL),
		p.ExpiresAt, p.Enabled, p.MaxProfiles,
		p.LastCheckAt, nullIfEmpty(p.LastCheckResult), p.LastCheckMessage, nullIfEmpty(p.Remark),
		p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func FindByID(id string) (model.Proxy, bool, error) {
	return findByID(database.DB(), id)
}

func findByID(db *gorm.DB, id string) (model.Proxy, bool, error) {
	row := queryRow(db, `SELECT `+proxyColumns+` FROM proxies WHERE id = ?`, id)
	p, err := scanProxy(row)
	if err == sql.ErrNoRows {
		return model.Proxy{}, false, nil
	}
	if err != nil {
		return model.Proxy{}, false, err
	}
	return p, true, nil
}

func List(filter ProxyFilter) ([]model.Proxy, error) {
	return list(database.DB(), filter)
}

func list(db *gorm.DB, filter ProxyFilter) ([]model.Proxy, error) {
	var conditions []string
	var args []interface{}

	if filter.TeamID != nil {
		conditions = append(conditions, "team_id = ?")
		args = append(args, *filter.TeamID)
	}
	if filter.Enabled != nil {
		conditions = append(conditions, "enabled = ?")
		args = append(args, *filter.Enabled)
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
		conditions = append(conditions, "(name LIKE ? OR host LIKE ? OR remark LIKE ? OR supplier LIKE ?)")
		s := "%" + filter.Search + "%"
		args = append(args, s, s, s, s)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf("SELECT %s FROM proxies%s ORDER BY created_at DESC LIMIT ? OFFSET ?", proxyColumns, where)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := queryRows(db, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Keep collection endpoints JSON-array shaped even when the database is empty.
	results := make([]model.Proxy, 0)
	for rows.Next() {
		p, err := scanProxy(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

// ListConnectionKeys 返回团队内所有代理的规范化连接标识。导入预览与提交共用
// 它，两者都不能用同一地址覆盖已有资源。
func ListConnectionKeys(teamID sharedidentity.TeamID) ([]ConnectionKey, error) {
	return listConnectionKeys(database.DB(), teamID)
}

func listConnectionKeys(db *gorm.DB, teamID sharedidentity.TeamID) ([]ConnectionKey, error) {
	rows, err := queryRows(db, `SELECT id, protocol, host, port FROM proxies WHERE team_id = ?`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]ConnectionKey, 0)
	for rows.Next() {
		var key ConnectionKey
		if err := rows.Scan(&key.ID, &key.Protocol, &key.Host, &key.Port); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func Update(p model.Proxy) error {
	return update(database.DB(), p)
}

func update(db *gorm.DB, p model.Proxy) error {
	err := execSQL(db,
		`UPDATE proxies SET name=?, source=?, protocol=?, host=?, port=?, username=?, password=?, region=?, supplier=?, tags=?, extract_url=?, expires_at=?, enabled=?, max_profiles=?, last_check_at=?, last_check_result=?, last_check_message=?, remark=?, updated_at=? WHERE id=?`,
		p.Name, p.Source, p.Protocol, p.Host, p.Port,
		nullIfEmpty(p.Username), nullIfEmpty(p.Password),
		nullIfEmpty(p.Region), nullIfEmpty(p.Supplier), nullIfEmpty(joinTags(p.Tags)), nullIfEmpty(p.ExtractURL),
		p.ExpiresAt, p.Enabled, p.MaxProfiles,
		p.LastCheckAt, nullIfEmpty(p.LastCheckResult), p.LastCheckMessage, nullIfEmpty(p.Remark),
		p.UpdatedAt, p.ID,
	)
	return err
}

// RecordCheckResult 仅在代理仍保持探测时的连接时写入结果。没有连接条件，
// 针对旧地址的慢检测会覆盖新地址的结果。
func RecordCheckResult(id, protocol, host string, port int, result, message string, checkedAt time.Time) (bool, error) {
	return recordCheckResult(database.DB(), id, protocol, host, port, result, message, checkedAt)
}

func recordCheckResult(db *gorm.DB, id, protocol, host string, port int, result, message string, checkedAt time.Time) (bool, error) {
	applied := db.Exec(
		`UPDATE proxies SET last_check_at = ?, last_check_result = ?, last_check_message = ?, updated_at = ? WHERE id = ? AND protocol = ? AND host = ? AND port = ?`,
		checkedAt, nullIfEmpty(result), nullIfEmpty(message), checkedAt, id, protocol, host, port,
	)
	return applied.RowsAffected > 0, applied.Error
}

func SetMaxProfiles(id string, maxProfileCount int, now time.Time) (bool, error) {
	return setMaxProfiles(database.DB(), id, maxProfileCount, now)
}

func setMaxProfiles(db *gorm.DB, id string, maxProfileCount int, now time.Time) (bool, error) {
	result := db.Exec(`UPDATE proxies SET max_profiles = ?, updated_at = ? WHERE id = ?`, maxProfileCount, now, id)
	return result.RowsAffected > 0, result.Error
}

func Delete(id string) error {
	return delete(database.DB(), id)
}

func delete(db *gorm.DB, id string) error {
	err := execSQL(db, `DELETE FROM proxies WHERE id = ?`, id)
	return err
}

// WithAssignmentLock 串行化同一代理的「先查后绑」。外部比特调用期间无法锁住
// 代理行，因此用绑定单条连接的 MySQL 命名锁，防止两个运营同时抢最后一个名额。
func WithAssignmentLock(proxyID string, fn func() error) error {
	return database.DB().Connection(func(tx *gorm.DB) error {
		name := "wt_proxy_assign:" + proxyID
		var acquired int
		if err := tx.Raw(`SELECT GET_LOCK(?, 10)`, name).Scan(&acquired).Error; err != nil {
			return err
		}
		if acquired != 1 {
			return ErrLockTimeout
		}
		defer func() { tx.Exec(`SELECT RELEASE_LOCK(?)`, name) }()
		return fn()
	})
}

func queryRow(db *gorm.DB, query string, args ...any) *sql.Row {
	return db.Raw(query, args...).Row()
}

func queryRows(db *gorm.DB, query string, args ...any) (*sql.Rows, error) {
	return db.Raw(query, args...).Rows()
}

func execSQL(db *gorm.DB, query string, args ...any) error {
	return db.Exec(query, args...).Error
}

// --- Scanner ---

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanProxy(row scannable) (model.Proxy, error) {
	var p model.Proxy
	var teamID int64
	var name, username, password, region, supplier, tagsJSON, extractURL, lastCheckResult, lastCheckMessage, remark sql.NullString
	var expiresAt, lastCheckAt sql.NullTime

	err := row.Scan(
		&p.ID, &teamID, &name, &p.Source, &p.Protocol, &p.Host, &p.Port,
		&username, &password, &region, &supplier, &tagsJSON,
		&extractURL, &expiresAt, &p.Enabled, &p.MaxProfiles,
		&lastCheckAt, &lastCheckResult, &lastCheckMessage, &remark,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return model.Proxy{}, err
	}

	p.TeamID = sharedidentity.TeamID(teamID)
	p.Name = name.String
	p.Username = username.String
	p.Password = password.String
	p.Region = region.String
	p.Supplier = supplier.String
	p.Tags = splitTags(tagsJSON.String)
	p.ExtractURL = extractURL.String
	if expiresAt.Valid {
		p.ExpiresAt = &expiresAt.Time
	}
	if lastCheckAt.Valid {
		p.LastCheckAt = &lastCheckAt.Time
	}
	p.LastCheckResult = lastCheckResult.String
	p.LastCheckMessage = lastCheckMessage.String
	p.Remark = remark.String

	return p, nil
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
