package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strconv"
	"strings"
	"time"
)

const importColumns = `id,user_id,request_key,platform,content_json,revision,created_at,updated_at`

func scanImport(row interface{ Scan(...any) error }) (b model.ImportBatch, e error) {
	var content []byte
	e = row.Scan(&b.ID, &b.UserID, &b.RequestKey, &b.Platform, &content, &b.Revision, &b.CreatedAt, &b.UpdatedAt)
	if e == nil {
		e = json.Unmarshal(content, &b.Content)
	}
	return
}
func quietImports(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}
func CreateImport(b model.ImportBatch) (model.ImportBatch, error) {
	return createImport(database.DB(), b)
}
func createImport(db *gorm.DB, b model.ImportBatch) (model.ImportBatch, error) {
	db = quietImports(db)
	raw, e := json.Marshal(b.Content)
	if e != nil {
		return b, e
	}
	e = db.Transaction(func(tx *gorm.DB) error {
		_, e := execSQL(tx, `INSERT INTO media_account_import_batches (user_id,request_key,platform,content_json,revision,created_at,updated_at) VALUES (?,?,?,?,0,?,?)`, b.UserID, b.RequestKey, b.Platform, string(raw), b.CreatedAt, b.UpdatedAt)
		if e != nil {
			return e
		}
		id, e := lastInsertID(tx)
		b.ID = strconv.FormatInt(id, 10)
		return e
	})
	if duplicateKey(e) {
		return scanImport(queryRow(db, `SELECT `+importColumns+` FROM media_account_import_batches WHERE user_id=? AND request_key=?`, b.UserID, b.RequestKey))
	}
	return b, e
}
func GetImport(id string, uid sharedidentity.UserID) (model.ImportBatch, error) {
	return getImport(database.DB(), id, uid)
}
func getImport(db *gorm.DB, id string, uid sharedidentity.UserID) (model.ImportBatch, error) {
	b, e := scanImport(queryRow(quietImports(db), `SELECT `+importColumns+` FROM media_account_import_batches WHERE id=? AND user_id=?`, id, uid))
	if errors.Is(e, sql.ErrNoRows) {
		e = model.ErrNotFound
	}
	return b, e
}
func ListImports(uid sharedidentity.UserID) ([]model.ImportBatch, error) {
	rows, e := queryRows(quietImports(database.DB()), `SELECT `+importColumns+` FROM media_account_import_batches WHERE user_id=? ORDER BY id DESC LIMIT 20`, uid)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.ImportBatch{}
	for rows.Next() {
		b, e := scanImport(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func MutateImport(id string, uid sharedidentity.UserID, fn func(*model.ImportBatch, model.ImportResources) error) (model.ImportBatch, error) {
	return mutateImport(database.DB(), id, uid, fn)
}
func mutateImport(db *gorm.DB, id string, uid sharedidentity.UserID, fn func(*model.ImportBatch, model.ImportResources) error) (out model.ImportBatch, err error) {
	err = quietImports(db).Transaction(func(tx *gorm.DB) error {
		b, e := scanImport(queryRow(tx, `SELECT `+importColumns+` FROM media_account_import_batches WHERE id=? AND user_id=? FOR UPDATE`, id, uid))
		if errors.Is(e, sql.ErrNoRows) {
			return model.ErrNotFound
		}
		if e != nil {
			return e
		}
		before, _ := json.Marshal(b.Content)
		rev := b.Revision
		if e = fn(&b, importResources{tx: tx, uid: uid}); e != nil {
			return e
		}
		after, e := json.Marshal(b.Content)
		if e != nil {
			return e
		}
		if string(before) != string(after) {
			n, e := execSQL(tx, `UPDATE media_account_import_batches SET content_json=?,revision=revision+1,updated_at=? WHERE id=? AND user_id=? AND revision=?`, string(after), time.Now().UTC(), id, uid, rev)
			if e != nil {
				return e
			}
			if n != 1 {
				return model.ErrImportConflict
			}
			b.Revision++
			b.UpdatedAt = time.Now().UTC()
		}
		out = b
		return nil
	})
	return
}

type importResources struct {
	tx  *gorm.DB
	uid sharedidentity.UserID
}

func (r importResources) ReserveProfile(id, op string, platform model.Platform, marker string) (model.ImportTarget, error) {
	var bit, remark, name string
	var lock, reserved sql.NullString
	var enabled bool
	e := queryRow(r.tx, `SELECT bp.bit_profile_id,bp.bit_name,bp.bit_remark,bp.cookie_operation_id,bp.reserved_proxy_id,bp.enabled FROM browser_profiles bp JOIN users u ON u.id=bp.user_id WHERE bp.id=? AND bp.user_id=? AND bp.bit_main_user_id=u.bit_main_user_id AND bp.local_state='active' FOR UPDATE`, id, r.uid).Scan(&bit, &name, &remark, &lock, &reserved, &enabled)
	if errors.Is(e, sql.ErrNoRows) {
		return model.ImportTarget{}, model.ErrProfileUnavailable
	}
	if e != nil {
		return model.ImportTarget{}, e
	}
	if bit == "" || !enabled || reserved.Valid || (lock.String != "" && lock.String != op) || (marker != "" && strings.TrimSpace(remark) != marker) {
		return model.ImportTarget{}, model.ErrImportConflict
	}
	var count int
	if e = queryRow(r.tx, `SELECT COUNT(*) FROM media_accounts WHERE browser_profile_id=?`, id).Scan(&count); e != nil {
		return model.ImportTarget{}, e
	}
	if count > 0 {
		return model.ImportTarget{}, model.ErrProfilePlatformTaken
	}
	_, e = execSQL(r.tx, `UPDATE browser_profiles SET cookie_operation_id=? WHERE id=?`, op, id)
	return model.ImportTarget{BitID: bit, Name: name}, e
}
func (r importResources) ReleaseProfile(id, op string) error {
	if id == "" {
		return nil
	}
	_, e := execSQL(r.tx, `UPDATE browser_profiles SET cookie_operation_id=NULL WHERE id=? AND user_id=? AND cookie_operation_id=?`, id, r.uid, op)
	return e
}
func (r importResources) FindIdentity(platform model.Platform, uid string) (string, error) {
	var id string
	e := queryRow(r.tx, `SELECT id FROM media_accounts WHERE user_id=? AND platform=? AND platform_account_id=? LIMIT 1`, r.uid, platform, uid).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		return "", nil
	}
	return id, e
}
func (r importResources) CreateAccount(a model.AccountRecord) (string, error) {
	id, e := createInTx(r.tx, a)
	if e != nil {
		return "", e
	}
	receipt, _ := json.Marshal(a.CookieUpdate)
	_, e = execSQL(r.tx, `UPDATE media_accounts SET before_cookie=?,cookie_update=?,last_check_status='completed',last_check_message=?,last_check_identity=? WHERE id=?`, a.BeforeCookie, string(receipt), "接入完成", marshalIdentity(a.LastCheckIdentity), id)
	return id, e
}
