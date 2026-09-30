-- 落盘命名改为 日期/游戏-素材ID：游戏名在任务创建时从素材投影解析并落列（filetransfer 模块
-- 不许读 production 表，见 architecture_test.go），租约再把 game_name 带给执行器。
-- 可选列：素材无游戏 / 查不到时为空，执行器回退「未分类」。
ALTER TABLE file_transfer_tasks ADD COLUMN game_name VARCHAR(64) NULL AFTER asset_title;
