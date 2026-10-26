// Package repository persists the M4 production facts with explicit SQL.
package repository

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("production record not found")

type CreateUsageInput struct {
	TeamID     identity.TeamID
	MaterialID int64
	UserID     identity.UserID
}

type MaterialFilter struct {
	TeamID      *identity.TeamID
	GameIDs     []string
	Search      string
	Limit       int
	GameID      string
	VideoStatus string
	SortBy      string
	SortOrder   string
	// UserID is the viewer whose personal relation and success facts the
	// `joined` / `my_usage` filters read. It is not a scope: team scope stays
	// TeamID, and this only decides which of the actor's own rows match.
	UserID identity.UserID
	// Joined, MyUsage and TeamUsage are the three independent usage dimensions
	// the library filters by (§5.3.6). Each is "", "used"/"unused" or
	// "joined"/"not_joined"; the service validates the values.
	Joined    string
	MyUsage   string
	TeamUsage string
}

// MaterialOutputFact is one (material, operator) group of successful outputs.
// OperatorID is nil for historical rows written before the operator fact
// existed; they count as output but cannot name a person.
type MaterialOutputFact struct {
	MaterialID  int64
	OperatorID  *int64
	OutputCount int64
}

// MaterialUsageRelation is the viewer's own relation to one material. Removal
// keeps the same row, so `removed` is a real answer rather than an absence.
type MaterialUsageRelation struct {
	MaterialID int64
	Status     model.MaterialUsageStatus
	CreatedAt  time.Time
}

const materialProjectionColumns = `m.id, m.team_id, s.game_id, m.source_content_id, s.platform_content_id, m.title, s.source_url, s.platform, s.author_name, s.cover_url, s.author_home_url, s.view_count, s.like_count, s.favorite_count, s.comment_count, s.share_count, s.published_at, m.video_status, m.status, m.source_object_key, m.video_size_bytes, m.video_sha256, m.video_media_json, m.video_error, m.video_prepared_at, m.created_at, m.updated_at`

// FindMaterial returns the material readiness projection only within the
// caller's team boundary. Game-range filtering is applied by the Service.
func FindMaterial(id int64, teamID identity.TeamID) (model.Material, bool, error) {
	return findMaterial(database.DB(), id, teamID)
}

func findMaterial(db *gorm.DB, id int64, teamID identity.TeamID) (model.Material, bool, error) {
	if id <= 0 || teamID <= 0 {
		return model.Material{}, false, fmt.Errorf("invalid material lookup")
	}
	var material model.Material
	err := scanMaterial(db.Raw(`SELECT `+materialProjectionColumns+` FROM materials m JOIN source_contents s ON s.id = m.source_content_id WHERE m.id = ? AND m.team_id = ?`, id, teamID).Row(), &material)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Material{}, false, nil
	}
	return material, err == nil, err
}

// FindMaterialUnscoped is for the production Service, which applies the
// actor's team and game scope before returning the material to a caller.
func FindMaterialUnscoped(id int64) (model.Material, bool, error) {
	return findMaterialUnscoped(database.DB(), id)
}

func findMaterialUnscoped(db *gorm.DB, id int64) (model.Material, bool, error) {
	if id <= 0 {
		return model.Material{}, false, fmt.Errorf("invalid material lookup")
	}
	var material model.Material
	err := scanMaterial(db.Raw(`SELECT `+materialProjectionColumns+` FROM materials m JOIN source_contents s ON s.id = m.source_content_id WHERE m.id = ?`, id).Row(), &material)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Material{}, false, nil
	}
	return material, err == nil, err
}

func ListMaterials(filter MaterialFilter) ([]model.Material, error) {
	return listMaterials(database.DB(), filter)
}

func listMaterials(db *gorm.DB, filter MaterialFilter) ([]model.Material, error) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, len(filter.GameIDs)+4)
	if filter.TeamID != nil {
		conditions = append(conditions, "m.team_id = ?")
		args = append(args, *filter.TeamID)
	}
	if len(filter.GameIDs) > 0 {
		placeholders := make([]string, 0, len(filter.GameIDs))
		for _, gameID := range filter.GameIDs {
			gameID = strings.TrimSpace(gameID)
			if gameID == "" {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, gameID)
		}
		if len(placeholders) > 0 {
			conditions = append(conditions, "s.game_id IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + search + "%"
		conditions = append(conditions, "(CAST(m.id AS CHAR) = ? OR m.title LIKE ? OR s.platform_content_id LIKE ? OR s.author_name LIKE ?)")
		args = append(args, search, like, like, like)
	}
	if filter.GameID != "" {
		conditions = append(conditions, "s.game_id = ?")
		args = append(args, filter.GameID)
	}
	if filter.VideoStatus != "" {
		conditions = append(conditions, "m.video_status = ?")
		args = append(args, filter.VideoStatus)
	}
	// The three usage dimensions are independent of each other and of the game
	// filter, so each is its own EXISTS and they combine with AND. None of them
	// rewrites material state: a given-up relation still counts as not joined,
	// while its successful outputs remain history.
	switch filter.Joined {
	case "joined":
		conditions = append(conditions, "EXISTS (SELECT 1 FROM material_usages u WHERE u.material_id = m.id AND u.user_id = ? AND u.status = 'active')")
		args = append(args, filter.UserID)
	case "not_joined":
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM material_usages u WHERE u.material_id = m.id AND u.user_id = ? AND u.status = 'active')")
		args = append(args, filter.UserID)
	}
	switch filter.MyUsage {
	case "used":
		conditions = append(conditions, "EXISTS (SELECT 1 FROM composite_outputs o WHERE o.material_id = m.id AND o.team_id = m.team_id AND o.operator_id = ?)")
		args = append(args, filter.UserID)
	case "unused":
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM composite_outputs o WHERE o.material_id = m.id AND o.team_id = m.team_id AND o.operator_id = ?)")
		args = append(args, filter.UserID)
	}
	// "someone used it" needs a known operator: a historical output without one
	// proves a success but names no responsible person, so it is counted in the
	// output total and not treated as this dimension's yes.
	switch filter.TeamUsage {
	case "used":
		conditions = append(conditions, "EXISTS (SELECT 1 FROM composite_outputs o WHERE o.material_id = m.id AND o.team_id = m.team_id AND o.operator_id IS NOT NULL)")
	case "unused":
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM composite_outputs o WHERE o.material_id = m.id AND o.team_id = m.team_id AND o.operator_id IS NOT NULL)")
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	query := `SELECT ` + materialProjectionColumns + ` FROM materials m JOIN source_contents s ON s.id = m.source_content_id`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	sortColumns := map[string]string{"id": "m.id", "published_at": "s.published_at", "created_at": "m.created_at", "updated_at": "m.updated_at"}
	sortColumn := sortColumns[filter.SortBy]
	if sortColumn == "" {
		sortColumn = "s.published_at"
	}
	sortOrder := "DESC"
	if strings.EqualFold(filter.SortOrder, "asc") {
		sortOrder = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s, m.id DESC LIMIT %d", sortColumn, sortOrder, limit)
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.Material, 0)
	for rows.Next() {
		var item model.Material
		if err := scanMaterial(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListMaterialOutputFacts groups successful outputs by material and operator.
// The join repeats the team on both sides so a material and an output that do
// not belong to the same team can never be added together: the same external
// work materialized in two teams is two rows with two separate histories.
func ListMaterialOutputFacts(materialIDs []int64) ([]MaterialOutputFact, error) {
	return listMaterialOutputFacts(database.DB(), materialIDs)
}

func listMaterialOutputFacts(db *gorm.DB, materialIDs []int64) ([]MaterialOutputFact, error) {
	ids := uniquePositiveIDs(materialIDs)
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := db.Raw(`SELECT o.material_id, o.operator_id, COUNT(*) FROM composite_outputs o JOIN materials m ON m.id = o.material_id AND m.team_id = o.team_id WHERE o.material_id IN (`+placeholders+`) GROUP BY o.material_id, o.operator_id ORDER BY o.material_id, o.operator_id`, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MaterialOutputFact, 0)
	for rows.Next() {
		var item MaterialOutputFact
		var operatorID sql.NullInt64
		if err := rows.Scan(&item.MaterialID, &operatorID, &item.OutputCount); err != nil {
			return nil, err
		}
		if operatorID.Valid {
			value := operatorID.Int64
			item.OperatorID = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListUsageRelations answers the viewer's own relation rows for the given
// materials, in both states. This is a read of the caller's rows, not a scope
// check: visibility is settled before the caller asks.
func ListUsageRelations(userID identity.UserID, materialIDs []int64) ([]MaterialUsageRelation, error) {
	return listUsageRelations(database.DB(), userID, materialIDs)
}

func listUsageRelations(db *gorm.DB, userID identity.UserID, materialIDs []int64) ([]MaterialUsageRelation, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid material usage user")
	}
	ids := uniquePositiveIDs(materialIDs)
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, userID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := db.Raw(`SELECT material_id, status, created_at FROM material_usages WHERE user_id = ? AND material_id IN (`+placeholders+`) ORDER BY material_id`, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MaterialUsageRelation, 0)
	for rows.Next() {
		var item MaterialUsageRelation
		if err := rows.Scan(&item.MaterialID, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// HasActiveUsage is the personal gate on the download route: only the current
// user's in-use relation lets a material be downloaded to this machine.
func HasActiveUsage(materialID int64, userID identity.UserID) (bool, error) {
	return hasActiveUsage(database.DB(), materialID, userID)
}

func hasActiveUsage(db *gorm.DB, materialID int64, userID identity.UserID) (bool, error) {
	if materialID <= 0 || userID <= 0 {
		return false, fmt.Errorf("invalid material usage lookup")
	}
	var one int
	err := db.Raw(`SELECT 1 FROM material_usages WHERE material_id = ? AND user_id = ? AND status = 'active' LIMIT 1`, materialID, userID).Row().Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func uniquePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

// CreateOrRestoreUsage atomically creates the sole usage row or restores its
// historical row. The unique key prevents concurrent clicks from creating two.
//
// The `created` answer is what tells a caller whether it should say 201 or 200,
// and it is read out of the statement rather than out of the row: an insert
// touches one row, a restore changes one, and a click on an already-active
// relation changes nothing at all. See the statement for why "changes nothing"
// is a real outcome here and not a coincidence.
func CreateOrRestoreUsage(input CreateUsageInput, now time.Time) (model.MaterialUsage, bool, error) {
	return createOrRestoreUsage(database.DB(), input, now)
}

func createOrRestoreUsage(db *gorm.DB, input CreateUsageInput, now time.Time) (model.MaterialUsage, bool, error) {
	if input.TeamID <= 0 || input.MaterialID <= 0 || input.UserID <= 0 {
		return model.MaterialUsage{}, false, fmt.Errorf("invalid material usage input")
	}
	var usage model.MaterialUsage
	created := false
	err := db.Transaction(func(tx *gorm.DB) error {
		// Every assignment that reads `status` is written **before** the assignment
		// that sets it, because MySQL evaluates these left to right and a later one
		// sees the new value. Reading `status` after it had been set to 'active'
		// would make both `IF(status = 'removed', ...)` branches take the
		// already-restored path: a restored row would keep its stale `removed_at`,
		// and no row would ever look unchanged.
		//
		// `updated_at` is conditional for the same reason it exists: if it were
		// `VALUES(updated_at)` then every click would count as a change,
		// `RowsAffected` would be 2 even when the relation was already active, and
		// a repeat click would answer 201 forever. `id = LAST_INSERT_ID(id)` is
		// how the read below finds the row in all three cases, including the one
		// that touched nothing.
		result := tx.Exec(`INSERT INTO material_usages (team_id, material_id, user_id, status, created_at, updated_at) VALUES (?,?,?,'active',?,?) ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id), removed_at = IF(status = 'removed', NULL, removed_at), updated_at = IF(status = 'removed', VALUES(updated_at), updated_at), status = IF(status = 'removed', 'active', status)`, input.TeamID, input.MaterialID, input.UserID, now, now)
		if result.Error != nil {
			return result.Error
		}
		created = result.RowsAffected != 0
		return scanUsage(tx.Raw(`SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE id = LAST_INSERT_ID()`).Row(), &usage)
	})
	return usage, created, err
}

// RemoveUsage preserves the historical usage row and its downstream task
// references; only its active relationship is removed.
func RemoveUsage(teamID identity.TeamID, materialID int64, userID identity.UserID, now time.Time) (bool, error) {
	return removeUsage(database.DB(), teamID, materialID, userID, now)
}

func removeUsage(db *gorm.DB, teamID identity.TeamID, materialID int64, userID identity.UserID, now time.Time) (bool, error) {
	if teamID <= 0 || materialID <= 0 || userID <= 0 {
		return false, fmt.Errorf("invalid material usage removal")
	}
	result := db.Exec(`UPDATE material_usages SET status = 'removed', removed_at = ?, updated_at = ? WHERE team_id = ? AND material_id = ? AND user_id = ? AND status = 'active'`, now, now, teamID, materialID, userID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// ListUsages returns every relation the user has with a material, in both states.
//
// It deliberately does not filter `status = 'active'`. A given-up relation is what
// the page renders as 已放弃 with a 恢复使用 button on it, so a list that dropped it
// would make the removal irreversible in the UI while the row sat in the database.
//
// Ordered by `created_at`, the timestamp the page shows as 加入时间, so that a row
// does not jump to the top the moment it is given up or restored — the user is
// usually about to click that same row again.
func ListUsages(userID identity.UserID) ([]model.MaterialUsage, error) {
	return listUsages(database.DB(), userID)
}

func listUsages(db *gorm.DB, userID identity.UserID) ([]model.MaterialUsage, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid material usage user")
	}
	rows, err := db.Raw(`SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE user_id = ? ORDER BY created_at DESC, id DESC`, userID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.MaterialUsage, 0)
	for rows.Next() {
		var usage model.MaterialUsage
		if err := scanUsage(rows, &usage); err != nil {
			return nil, err
		}
		items = append(items, usage)
	}
	return items, rows.Err()
}

func FindUsageForUser(usageID int64, userID identity.UserID) (model.MaterialUsage, bool, error) {
	return findUsageForUser(database.DB(), usageID, userID)
}

func findUsageForUser(db *gorm.DB, usageID int64, userID identity.UserID) (model.MaterialUsage, bool, error) {
	if usageID <= 0 || userID <= 0 {
		return model.MaterialUsage{}, false, fmt.Errorf("invalid material usage lookup")
	}
	var usage model.MaterialUsage
	err := scanUsage(db.Raw(`SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE id = ? AND user_id = ?`, usageID, userID).Row(), &usage)
	if errors.Is(err, sql.ErrNoRows) {
		return model.MaterialUsage{}, false, nil
	}
	return usage, err == nil, err
}

// FindUsage is the unscoped, historical read the compose-output trace needs:
// the usage a task was created from is a fact about that task, not a relation
// the reading actor holds.
func FindUsage(usageID int64) (model.MaterialUsage, bool, error) {
	if usageID <= 0 {
		return model.MaterialUsage{}, false, nil
	}
	var usage model.MaterialUsage
	err := scanUsage(database.DB().Raw(`SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE id = ?`, usageID).Row(), &usage)
	if errors.Is(err, sql.ErrNoRows) {
		return model.MaterialUsage{}, false, nil
	}
	return usage, err == nil, err
}

func RemoveUsageByID(usageID int64, userID identity.UserID, now time.Time) (bool, error) {
	return removeUsageByID(database.DB(), usageID, userID, now)
}

func removeUsageByID(db *gorm.DB, usageID int64, userID identity.UserID, now time.Time) (bool, error) {
	if usageID <= 0 || userID <= 0 {
		return false, fmt.Errorf("invalid material usage removal")
	}
	result := db.Exec(`UPDATE material_usages SET status = 'removed', removed_at = ?, updated_at = ? WHERE id = ? AND user_id = ? AND status = 'active'`, now, now, usageID, userID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// RestoreUsageByID is the inverse of RemoveUsageByID: it puts a given-up relation
// back into use and clears the moment it was given up in the same statement.
//
// Clearing `removed_at` is not cosmetic. The row is what 使用情况 renders, so a
// relation that reads 使用中 while still carrying its removal timestamp would show
// 已放弃's history on something that is in use again.
//
// The `status = 'removed'` guard is what keeps two concurrent restores from both
// writing, and it is asserted as text in the tests for the same reason the
// preparation guards are: sqlmock cannot execute it, and a statement without the
// guard would match a mock that only checked the table.
//
// No flag comes back. The only outcome zero affected rows can describe — the row
// was already `active` when the statement ran — is the state the caller asked for,
// so the caller reads it as success either way and a bool would be a value nobody
// branches on.
func RestoreUsageByID(usageID int64, userID identity.UserID, now time.Time) error {
	return restoreUsageByID(database.DB(), usageID, userID, now)
}

func restoreUsageByID(db *gorm.DB, usageID int64, userID identity.UserID, now time.Time) error {
	if usageID <= 0 || userID <= 0 {
		return fmt.Errorf("invalid material usage restoration")
	}
	return db.Exec(`UPDATE material_usages SET status = 'active', removed_at = NULL, updated_at = ? WHERE id = ? AND user_id = ? AND status = 'removed'`, now, usageID, userID).Error
}

// MarkVideoPreparing moves the material's readiness projection to `downloading`,
// and only out of the states a preparation may legitimately start from.
//
// The guard is the whole point rather than a nicety: `ready` is the projection's
// promise that the object key, the size and the hash were written together, and a
// click that read the row before a preparation finished must not be able to take
// that promise back. The one `ready` row this statement does take back is one
// carrying no object key at all: it promises a video no key names, and the same
// statement is what moves it towards a usable one. The caller is told which of the
// two happened through the returned flag instead of being left to assume it won.
//
// The previous failure's message is cleared with the status, because it belongs to
// an attempt that is no longer the current one and the new attempt's outcome is not
// known yet; the task row keeps the old code and message for the record.
func MarkVideoPreparing(teamID identity.TeamID, materialID int64, now time.Time) (bool, error) {
	return markVideoPreparing(database.DB(), teamID, materialID, now)
}

const markVideoPreparingSQL = `UPDATE materials SET video_status = 'downloading', video_error = '', updated_at = ? WHERE id = ? AND team_id = ? AND (video_status IN ('not_downloaded', 'failed') OR (video_status = 'ready' AND COALESCE(source_object_key, '') = ''))`

func markVideoPreparing(db *gorm.DB, teamID identity.TeamID, materialID int64, now time.Time) (bool, error) {
	if teamID <= 0 || materialID <= 0 {
		return false, fmt.Errorf("invalid material preparation marker")
	}
	result := db.Exec(markVideoPreparingSQL, now, materialID, teamID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// MarkVideoRepreparing moves a `ready` material back to `downloading` when the
// object it names has been observed missing.
//
// This is a different act from `MarkVideoPreparing`, and the predicate says why in
// two clauses. `ready` is normally taken back by nothing, because the row's promise
// is that its key is fetchable; this statement exists for the case where that
// promise has been observed false. The caller therefore has to name the exact key
// it found missing, and the row has to still carry that key with the same
// `video_prepared_at` the caller read. Both together keep a slower reader from
// taking back a newer preparation's result: if a concurrent attempt finished in
// between, either the key differs (different bytes, different key) or the prepared
// timestamp differs, and zero rows come back.
//
// `video_prepared_at <=> ?` is MySQL's null-safe equality, so a legacy row with a
// NULL timestamp matches a caller that read NULL.
//
// An empty expected key is refused in Go rather than matched as a special case: a
// keyless `ready` row is `MarkVideoPreparing`'s business, and letting this
// statement also cover it would make two entry points with different guards mean
// the same thing.
func MarkVideoRepreparing(teamID identity.TeamID, materialID int64, expectedObjectKey string, expectedPreparedAt *time.Time, now time.Time) (bool, error) {
	return markVideoRepreparing(database.DB(), teamID, materialID, expectedObjectKey, expectedPreparedAt, now)
}

const markVideoRepreparingSQL = `UPDATE materials SET video_status = 'downloading', video_error = '', updated_at = ? WHERE id = ? AND team_id = ? AND video_status = 'ready' AND COALESCE(source_object_key, '') = ? AND video_prepared_at <=> ?`

func markVideoRepreparing(db *gorm.DB, teamID identity.TeamID, materialID int64, expectedObjectKey string, expectedPreparedAt *time.Time, now time.Time) (bool, error) {
	expectedObjectKey = strings.TrimSpace(expectedObjectKey)
	if teamID <= 0 || materialID <= 0 || expectedObjectKey == "" {
		return false, fmt.Errorf("invalid material repreparation marker")
	}
	result := db.Exec(markVideoRepreparingSQL, now, materialID, teamID, expectedObjectKey, expectedPreparedAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// VideoFacts is the whole of what a verified preparation writes onto a material.
//
// It is one value rather than four arguments because the projection's promise is
// that these facts arrived together: a key without its hash, or a size without its
// key, is a row that says `ready` about a video nobody can fetch. Collecting them
// here is what lets the write refuse an incomplete set.
type VideoFacts struct {
	ObjectKey string
	SizeBytes int64
	SHA256    string
	// Media is the probe's reading of the file, as JSON, and may be nil: a file
	// whose bytes are verified but whose container yields no facts is prepared, not
	// failed — the probe is best-effort and says so in its own package.
	Media []byte
}

// MarkVideoReady writes the facts of a verified prepared source and moves the
// material's projection to `ready`.
//
// There is deliberately no status predicate on the update, unlike
// `MarkVideoPreparing` above. A preparation may legitimately finish from
// `downloading`, from `failed` (a retry that worked), or from `not_downloaded` (a
// task created by a path that never moved the projection), so a predicate would
// have to name every state and would refuse the ones it forgot. What makes the
// write safe instead is content addressing: the key is derived from the bytes'
// own digest, so two attempts that finish produce the same key, and a retry that
// completes after a slower earlier attempt leaves the same row either way. The
// refusal that matters — a `ready` material being taken back — is on the path that
// can do it, `MarkVideoPreparing`.
//
// The facts are checked for completeness rather than trusted. This is the seam
// where an unverified download would become a durable claim, and a row that says
// `ready` with no key, no size or a malformed hash is worse than one that says
// `failed`: nothing downstream can tell it apart from a prepared video.
func MarkVideoReady(teamID identity.TeamID, materialID int64, facts VideoFacts, now time.Time) (bool, error) {
	return markVideoReady(database.DB(), teamID, materialID, facts, now)
}

const markVideoReadySQL = `UPDATE materials SET video_status = 'ready', source_object_key = ?, video_size_bytes = ?, video_sha256 = ?, video_media_json = ?, video_error = '', video_prepared_at = ?, updated_at = ? WHERE id = ? AND team_id = ?`

func markVideoReady(db *gorm.DB, teamID identity.TeamID, materialID int64, facts VideoFacts, now time.Time) (bool, error) {
	if teamID <= 0 || materialID <= 0 {
		return false, fmt.Errorf("invalid material readiness marker")
	}
	if err := validateVideoFacts(facts); err != nil {
		return false, err
	}
	result := db.Exec(markVideoReadySQL, strings.TrimSpace(facts.ObjectKey), facts.SizeBytes, strings.ToLower(strings.TrimSpace(facts.SHA256)), mediaJSON(facts.Media), now, now, materialID, teamID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// MarkVideoNotPrepared takes the readiness projection back to `not_downloaded`
// when the preparation it was waiting on was cancelled.
//
// The predicate is the whole function. `downloading` is the only state that claims
// a preparation is in flight, and the only state this write is about: `ready` must
// never be taken back — the object the row names is fetchable, and a cancellation
// that arrives late would otherwise leave a prepared material reading as though
// nothing had been done — while `not_downloaded` and `failed` already say what
// this write would. Both of those readings are `false`, which is why the caller is
// told the count instead of an error: a cancellation that finds nothing to take
// back has not failed.
//
// `video_error` is cleared with the status, as `MarkVideoPreparing` clears it on
// the way in: the row leaves `downloading` with every field that describes a
// preparation attempt reset, so a message from an attempt that is over cannot
// outlive the state it was written for.
func MarkVideoNotPrepared(teamID identity.TeamID, materialID int64, now time.Time) (bool, error) {
	return markVideoNotPrepared(database.DB(), teamID, materialID, now)
}

const markVideoNotPreparedSQL = `UPDATE materials SET video_status = 'not_downloaded', video_error = '', updated_at = ? WHERE id = ? AND team_id = ? AND video_status = 'downloading'`

func markVideoNotPrepared(db *gorm.DB, teamID identity.TeamID, materialID int64, now time.Time) (bool, error) {
	if teamID <= 0 || materialID <= 0 {
		return false, fmt.Errorf("invalid material unprepared marker")
	}
	result := db.Exec(markVideoNotPreparedSQL, now, materialID, teamID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// MarkVideoFailed records that a preparation did not produce a verified source.
//
// The update refuses a material that is `ready` with an object key, and that
// predicate is the reason this function exists rather than being folded into the
// caller: a retry that fails after a slower earlier attempt succeeded would
// otherwise take a material that has a fetchable video back to `failed`, and the
// row would say the video is unavailable while the object it names sits in the
// bucket. The one `ready` row it does take back is the keyless one, which promises
// nothing fetchable; writing `failed` with the reason is strictly more information
// than leaving it keyless and `ready`.
func MarkVideoFailed(teamID identity.TeamID, materialID int64, message string, now time.Time) (bool, error) {
	return markVideoFailed(database.DB(), teamID, materialID, message, now)
}

const markVideoFailedSQL = `UPDATE materials SET video_status = 'failed', video_error = ?, updated_at = ? WHERE id = ? AND team_id = ? AND (video_status <> 'ready' OR COALESCE(source_object_key, '') = '')`

func markVideoFailed(db *gorm.DB, teamID identity.TeamID, materialID int64, message string, now time.Time) (bool, error) {
	if teamID <= 0 || materialID <= 0 {
		return false, fmt.Errorf("invalid material failure marker")
	}
	result := db.Exec(markVideoFailedSQL, boundedVideoError(message), now, materialID, teamID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// videoErrorLimit is the width of `materials.video_error`, and the message is cut
// to it here rather than by the server.
//
// MySQL runs in strict mode, so a longer message is error 1406 and the statement
// fails — which would lose the reason a material failed precisely when it is most
// wanted, and would leave the projection saying `downloading` with a task that has
// already given up. The cut is marked so that a reader can see the message was
// longer rather than being told a truncated sentence is the whole of it.
const videoErrorLimit = 500

func boundedVideoError(message string) string {
	message = strings.TrimSpace(message)
	runes := []rune(message)
	if len(runes) <= videoErrorLimit {
		return message
	}
	return string(runes[:videoErrorLimit-1]) + "…"
}

func validateVideoFacts(facts VideoFacts) error {
	if strings.TrimSpace(facts.ObjectKey) == "" {
		return errors.New("a prepared source must name its object key")
	}
	if facts.SizeBytes <= 0 {
		return fmt.Errorf("a prepared source must have a positive size, got %d", facts.SizeBytes)
	}
	digest := strings.ToLower(strings.TrimSpace(facts.SHA256))
	if len(digest) != 64 {
		return fmt.Errorf("a prepared source must carry a 64-character sha256, got %d characters", len(digest))
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return fmt.Errorf("a prepared source's sha256 is not hexadecimal: %w", err)
	}
	if len(facts.Media) > 0 && !json.Valid(facts.Media) {
		return errors.New("a prepared source's media summary is not JSON")
	}
	return nil
}

// mediaJSON passes the probe's JSON through, and passes nothing at all when there
// is none: `[]byte(nil)` into a JSON column would write the four characters
// `null` as a document rather than writing SQL NULL, and those two are different
// answers to "was the file probed".
func mediaJSON(media []byte) any {
	if len(media) == 0 {
		return nil
	}
	return string(media)
}

type rowScanner interface{ Scan(...any) error }

func scanUsage(row rowScanner, usage *model.MaterialUsage) error {
	var teamID, userID int64
	var status string
	var removedAt sql.NullTime
	if err := row.Scan(&usage.ID, &teamID, &usage.MaterialID, &userID, &status, &removedAt, &usage.CreatedAt, &usage.UpdatedAt); err != nil {
		return err
	}
	usage.TeamID = identity.TeamID(teamID)
	usage.UserID = identity.UserID(userID)
	usage.Status = model.MaterialUsageStatus(status)
	if removedAt.Valid {
		usage.RemovedAt = &removedAt.Time
	}
	return nil
}

func scanMaterial(row rowScanner, material *model.Material) error {
	var teamID int64
	var gameID, sourceURL, authorName, coverURL, authorHomeURL, objectKey, sha256, videoError sql.NullString
	var platformContentID sql.NullString
	var publishedAt, preparedAt sql.NullTime
	var size sql.NullInt64
	var status string
	// The library lifecycle's column is NOT NULL, so this is a plain string.
	var materialStatus string
	var mediaJSON []byte
	if err := row.Scan(&material.ID, &teamID, &gameID, &material.SourceContentID, &platformContentID, &material.Title, &sourceURL, &material.Platform, &authorName, &coverURL, &authorHomeURL, &material.ViewCount, &material.LikeCount, &material.FavoriteCount, &material.CommentCount, &material.ShareCount, &publishedAt, &status, &materialStatus, &objectKey, &size, &sha256, &mediaJSON, &videoError, &preparedAt, &material.CreatedAt, &material.UpdatedAt); err != nil {
		return err
	}
	material.PlatformContentID = platformContentID.String
	material.TeamID = identity.TeamID(teamID)
	material.SourceURL = sourceURL.String
	material.AuthorName = authorName.String
	material.CoverURL = coverURL.String
	material.AuthorHomeURL = authorHomeURL.String
	material.VideoStatus = model.VideoStatus(status)
	material.Status = model.MaterialStatus(materialStatus)
	material.SourceObjectKey = objectKey.String
	material.VideoSHA256 = sha256.String
	material.VideoError = videoError.String
	if gameID.Valid {
		value := gameID.String
		material.GameID = &value
	}
	if publishedAt.Valid {
		material.PublishedAt = &publishedAt.Time
	}
	if size.Valid {
		value := size.Int64
		material.VideoSizeBytes = &value
	}
	if preparedAt.Valid {
		material.VideoPreparedAt = &preparedAt.Time
	}
	if len(mediaJSON) > 0 {
		if err := json.Unmarshal(mediaJSON, &material.VideoMedia); err != nil {
			return fmt.Errorf("decode material media summary: %w", err)
		}
	}
	return nil
}
