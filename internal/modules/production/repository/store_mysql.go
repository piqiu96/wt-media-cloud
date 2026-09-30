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
	TeamID  *identity.TeamID
	GameIDs []string
	Search  string
	Limit   int
}

const materialProjectionColumns = `m.id, m.team_id, s.game_id, m.source_content_id, s.platform_content_id, m.title, s.source_url, s.platform, s.author_name, s.cover_url, s.author_home_url, s.view_count, s.like_count, s.favorite_count, s.comment_count, s.share_count, s.published_at, m.video_status, m.source_object_key, m.video_size_bytes, m.video_sha256, m.video_media_json, m.video_error, m.video_prepared_at, m.created_at, m.updated_at`

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
		conditions = append(conditions, "(m.title LIKE ? OR s.platform_content_id LIKE ? OR s.author_name LIKE ?)")
		args = append(args, like, like, like)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	query := `SELECT ` + materialProjectionColumns + ` FROM materials m JOIN source_contents s ON s.id = m.source_content_id`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY m.id DESC LIMIT %d", limit)
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

func ListActiveUsages(userID identity.UserID) ([]model.MaterialUsage, error) {
	return listActiveUsages(database.DB(), userID)
}

func listActiveUsages(db *gorm.DB, userID identity.UserID) ([]model.MaterialUsage, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid material usage user")
	}
	rows, err := db.Raw(`SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE user_id = ? AND status = 'active' ORDER BY updated_at DESC, id DESC`, userID).Rows()
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

// MarkVideoPreparing moves the material's readiness projection to `downloading`,
// and only out of the two states a preparation may legitimately start from.
//
// The guard is the whole point rather than a nicety: `ready` is the projection's
// promise that the object key, the size and the hash were written together, and a
// click that read the row before a preparation finished must not be able to take
// that promise back. The caller is told which of the two happened through the
// returned flag instead of being left to assume it won.
//
// The previous failure's message is cleared with the status, because it belongs to
// an attempt that is no longer the current one and the new attempt's outcome is not
// known yet; the task row keeps the old code and message for the record.
func MarkVideoPreparing(teamID identity.TeamID, materialID int64, now time.Time) (bool, error) {
	return markVideoPreparing(database.DB(), teamID, materialID, now)
}

const markVideoPreparingSQL = `UPDATE materials SET video_status = 'downloading', video_error = '', updated_at = ? WHERE id = ? AND team_id = ? AND video_status IN ('not_downloaded', 'failed')`

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
// The update refuses a material that is already `ready`, and that predicate is the
// reason this function exists rather than being folded into the caller: a retry
// that fails after a slower earlier attempt succeeded would otherwise take a
// material that has a fetchable video back to `failed`, and the row would say the
// video is unavailable while the object it names sits in the bucket.
func MarkVideoFailed(teamID identity.TeamID, materialID int64, message string, now time.Time) (bool, error) {
	return markVideoFailed(database.DB(), teamID, materialID, message, now)
}

const markVideoFailedSQL = `UPDATE materials SET video_status = 'failed', video_error = ?, updated_at = ? WHERE id = ? AND team_id = ? AND video_status <> 'ready'`

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
	var mediaJSON []byte
	if err := row.Scan(&material.ID, &teamID, &gameID, &material.SourceContentID, &platformContentID, &material.Title, &sourceURL, &material.Platform, &authorName, &coverURL, &authorHomeURL, &material.ViewCount, &material.LikeCount, &material.FavoriteCount, &material.CommentCount, &material.ShareCount, &publishedAt, &status, &objectKey, &size, &sha256, &mediaJSON, &videoError, &preparedAt, &material.CreatedAt, &material.UpdatedAt); err != nil {
		return err
	}
	material.PlatformContentID = platformContentID.String
	material.TeamID = identity.TeamID(teamID)
	material.SourceURL = sourceURL.String
	material.AuthorName = authorName.String
	material.CoverURL = coverURL.String
	material.AuthorHomeURL = authorHomeURL.String
	material.VideoStatus = model.VideoStatus(status)
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
