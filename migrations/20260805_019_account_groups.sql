-- 20260805_019_account_groups.sql
-- M2-B 账号收口 Task5：账号组（可保存筛选，PRD 3.3.6）
-- 用户将一组筛选条件保存为命名账号组，一键应用筛选目标账号；
-- 发布/互动（M6/M8）将依赖它筛选目标账号。

CREATE TABLE account_groups (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,  -- 主键一律自增
    user_id    BIGINT UNSIGNED NOT NULL,   -- 创建者（私有组），关联 users(id)
    team_id    BIGINT UNSIGNED NULL,       -- 团队范围，关联 operation_teams(id)
    name       VARCHAR(128)   NOT NULL,    -- 账号组名称
    filters    JSON           NOT NULL,    -- 筛选条件（AccountFilter 子集，均可 null）
    sort_order INT            NOT NULL DEFAULT 0,
    created_at DATETIME(6)    NOT NULL,
    updated_at DATETIME(6)    NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uq_account_groups_user_name (user_id, name),
    KEY idx_account_groups_user (user_id),
    CONSTRAINT fk_account_groups_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT fk_account_groups_team FOREIGN KEY (team_id) REFERENCES operation_teams(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
