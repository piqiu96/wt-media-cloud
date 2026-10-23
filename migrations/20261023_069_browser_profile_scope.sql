-- browser_profiles 按系统用户与其绑定的 BitBrowser 主账号双作用域。
-- 同一 bit_profile_id 在不同主账号下可以合法共存；切换主账号既不冲突也不迁移行：
-- 旧的全局唯一键让第二个主账号无法写入，两列键也仍然阻止共用同一主账号的两个用户
-- 各自保留自己的行。
--
-- 存在重复行时新唯一键无法创建，因此先用守卫让迁移以明确的错误停下，
-- 而不是让 ALTER 报一个 MySQL 的重复键错误。
CREATE TEMPORARY TABLE m2_f_profile_scope_guard (
    valid TINYINT NOT NULL,
    CONSTRAINT chk_m2_f_profile_scope_guard CHECK (valid = 1)
);

INSERT INTO m2_f_profile_scope_guard (valid)
SELECT IF(
    (SELECT COUNT(*) FROM (
        SELECT user_id, main_user_id, bit_profile_id
        FROM browser_profiles
        GROUP BY user_id, main_user_id, bit_profile_id
        HAVING COUNT(*) > 1
    ) duplicates) = 0,
    1,
    0
);

DROP TEMPORARY TABLE m2_f_profile_scope_guard;

ALTER TABLE browser_profiles
    DROP INDEX uq_browser_profiles_bit_profile,
    DROP INDEX uq_browser_profiles_user_bit_profile,
    ADD UNIQUE KEY uq_browser_profiles_user_main_bit (user_id, main_user_id, bit_profile_id),
    ADD KEY idx_browser_profiles_user_main (user_id, main_user_id);
