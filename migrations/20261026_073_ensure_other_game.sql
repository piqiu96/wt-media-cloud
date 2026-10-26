-- 内容池与挖掘策略的 game_id 必填、默认 other（PRD 第四章 §4.7.2）。
-- 已有环境可能由人工创建过 other；重复执行不覆盖它的名称和状态。
INSERT IGNORE INTO operation_games (id, name, status, remark, created_at, updated_at)
VALUES ('other', '其他', 'enabled', '内容池与挖掘策略的默认游戏', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6));
