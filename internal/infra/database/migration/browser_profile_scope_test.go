package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 迁移 069 把 browser_profiles 的单用户唯一约束换成产品需要的双作用域：
// 一个系统用户 + 一个 BitBrowser 主账号 + 一个 BitBrowser Profile。
// 全局键必须先删除——只要它还在，第二个主账号就无法保存自己的那份 Profile。
func TestBrowserProfileScopeMigration(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "migrations", "20261023_069_browser_profile_scope.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(content)

	for _, fragment := range []string{
		"DROP INDEX uq_browser_profiles_bit_profile",
		"DROP INDEX uq_browser_profiles_user_bit_profile",
		"ADD UNIQUE KEY uq_browser_profiles_user_main_bit (user_id, main_user_id, bit_profile_id)",
		"ADD KEY idx_browser_profiles_user_main (user_id, main_user_id)",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}

	// 新键下的重复行会让 ALTER 报 MySQL 的通用重复键错误；守卫负责指出真正的问题。
	guardAt := strings.Index(sql, "chk_m2_f_profile_scope_guard")
	if guardAt < 0 {
		t.Fatal("migration has no duplicate guard")
	}
	if alterAt := strings.Index(sql, "ALTER TABLE browser_profiles"); alterAt < guardAt {
		t.Fatal("the duplicate guard must run before the ALTER")
	}
	if !strings.Contains(sql, "GROUP BY user_id, main_user_id, bit_profile_id") {
		t.Fatal("the guard must count duplicates under the new key")
	}
	if strings.Contains(sql, "CREATE TABLE") {
		t.Fatal("scoping uniqueness must not create tables")
	}
}
