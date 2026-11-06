package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

// CutoverBackup contains complete affected rows, including the source pointers
// that are cleared rather than deleted. It is private operator recovery data.
type CutoverBackup struct {
	Version       int                         `json:"version"`
	CreatedAt     time.Time                   `json:"created_at"`
	PrimaryBucket string                      `json:"primary_bucket"`
	Rows          map[string][]map[string]any `json:"rows"`
}
type cutoverSelection struct {
	table, predicate string
	args             []any
}

// RetireLegacyResources is an explicit maintenance operation, never a migration
// or startup hook. Locks, inventory, protection checks and deletion share one
// transaction; a failed durable backup aborts the operation before any mutation.
func RetireLegacyResources(ctx context.Context, apply bool, backup func(CutoverBackup) error) (map[string]int, error) {
	return retireLegacyResources(database.DB().WithContext(ctx), apply, backup)
}
func retireLegacyResources(db *gorm.DB, apply bool, save func(CutoverBackup) error) (map[string]int, error) {
	counts := map[string]int{}
	err := db.Transaction(func(tx *gorm.DB) error {
		b := CutoverBackup{Version: 1, CreatedAt: time.Now().UTC(), PrimaryBucket: "wt-media", Rows: map[string][]map[string]any{}}

		read := func(table, predicate string, args ...any) error {
			var rows []map[string]any
			if err := tx.Table(table).Clauses(clause.Locking{Strength: "UPDATE"}).Where(predicate, args...).Find(&rows).Error; err != nil {
				return err
			}
			b.Rows[table] = rows

			counts[table] = len(rows)
			return nil
		}
		ids := func(table string) []any {
			var v []any
			for _, r := range b.Rows[table] {
				v = append(v, r["id"])
			}
			return v
		}
		codes := []string{"basic_cut", "guide_insert", "guide_pip"}
		if err := read("compose_templates", "code IN ?", codes); err != nil {
			return err
		}
		if err := read("compose_strategies", "compose_template_id IN ?", ids("compose_templates")); err != nil {
			return err
		}
		oldLocation := "COALESCE(source_object_key,'') <> '' AND source_object_key NOT LIKE 's3://wt-media/%'"
		if err := read("materials", oldLocation); err != nil {
			return err
		}
		if err := read("compose_tasks", "template_code IN ? OR compose_strategy_id IN ? OR material_id IN ? OR (COALESCE(output_object_key,'') <> '' AND output_object_key NOT LIKE 's3://wt-media/%')", codes, ids("compose_strategies"), ids("materials")); err != nil {
			return err
		}
		if err := read("composite_outputs", "compose_task_id IN ? OR compose_strategy_id IN ? OR material_id IN ? OR (object_key <> '' AND object_key NOT LIKE 's3://wt-media/%')", ids("compose_tasks"), ids("compose_strategies"), ids("materials")); err != nil {
			return err
		}
		// A current object or unfinished run is never collateral for template removal.
		for _, table := range []string{"materials", "composite_outputs", "compose_tasks"} {
			for _, r := range b.Rows[table] {
				for _, key := range []string{"source_object_key", "object_key", "output_object_key"} {
					if strings.HasPrefix(cutoverString(r[key]), "s3://wt-media/") {
						return fmt.Errorf("refusing to remove current asset: %s %v", table, r["id"])
					}
				}
				if table == "compose_tasks" {
					status := cutoverString(r["status"])
					if status == "pending" || status == "running" || status == "uncertain" {
						return fmt.Errorf("unfinished retired task %v", r["id"])
					}
				}
			}
		}
		if err := read("material_usages", "material_id IN ?", ids("materials")); err != nil {
			return err
		}
		// Batches can contain surviving tasks: select only empty-after-cutover batches.
		if err := read("compose_task_batches", "(compose_strategy_id IN ? OR id IN (SELECT batch_id FROM compose_tasks WHERE id IN ?)) AND NOT EXISTS (SELECT 1 FROM compose_tasks t WHERE t.batch_id=compose_task_batches.id AND t.id NOT IN ?)", ids("compose_strategies"), ids("compose_tasks"), ids("compose_tasks")); err != nil {
			return err
		}
		if err := read("compose_schedule_runs", "compose_strategy_id IN ? OR batch_id IN ?", ids("compose_strategies"), ids("compose_task_batches")); err != nil {
			return err
		}
		if err := read("compose_task_attempts", "task_id IN ?", ids("compose_tasks")); err != nil {
			return err
		}
		if err := read("compose_output_allocations", "task_id IN ?", ids("compose_tasks")); err != nil {
			return err
		}
		transferWhere := "(asset_type='material' AND asset_id IN ?) OR (asset_type='composite_output' AND asset_id IN ?) OR (COALESCE(source_object_key,'') <> '' AND source_object_key NOT LIKE 's3://wt-media/%')"
		var deps []any
		for {
			if err := read("file_transfer_tasks", transferWhere+" OR dependency_task_id IN ?", ids("materials"), ids("composite_outputs"), deps); err != nil {
				return err
			}
			next := ids("file_transfer_tasks")
			if len(next) == len(deps) {
				break
			}
			deps = next
		}
		for _, r := range b.Rows["file_transfer_tasks"] {
			if cutoverString(r["status"]) == "running" || strings.HasPrefix(cutoverString(r["source_object_key"]), "s3://wt-media/") {
				return fmt.Errorf("protected transfer %v", r["id"])
			}
		}
		if err := read("source_contents", "material_id IN ?", ids("materials")); err != nil {
			return err
		}
		if save == nil {
			return errors.New("a durable backup callback is required")
		}
		if err := save(b); err != nil {
			return err
		}
		if !apply {
			return nil
		}
		// Foreign keys stay enabled. Unexpected dependencies fail and roll back.
		for _, table := range []string{"file_transfer_tasks", "composite_outputs", "compose_task_attempts", "compose_output_allocations", "compose_tasks", "compose_schedule_runs", "compose_task_batches", "material_usages", "materials", "compose_strategies", "compose_templates"} {
			sel := cutoverSelection{table: table, predicate: "id IN ?", args: []any{ids(table)}}
			if table == "compose_schedule_runs" {
				var keys []any
				for _, r := range b.Rows[table] {
					keys = append(keys, r["run_key"])
				}
				sel.predicate = "run_key IN ?"
				sel.args = []any{keys}
			}
			if table == "compose_task_attempts" {
				sel.predicate = "task_id IN ?"
				sel.args = []any{ids("compose_tasks")}
			}
			if table == "file_transfer_tasks" {
				if err := tx.Table(table).Where(sel.predicate, sel.args...).Update("dependency_task_id", nil).Error; err != nil {
					return err
				}
			}
			if table == "materials" {
				if err := tx.Table("source_contents").Where("material_id IN ?", ids("materials")).Update("material_id", nil).Error; err != nil {
					return err
				}
			}
			result := tx.Table(table).Where(sel.predicate, sel.args...).Delete(nil)
			if result.Error != nil {
				return fmt.Errorf("delete %s: %w", table, result.Error)
			}
			if result.RowsAffected != int64(counts[table]) {
				return fmt.Errorf("changed cleanup selection for %s", table)
			}
		}
		return nil
	})
	return counts, err
}
func cutoverString(v any) string {
	if v == nil {
		return ""
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprint(v)
}
