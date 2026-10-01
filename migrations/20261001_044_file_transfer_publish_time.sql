-- 落盘命名改为 日期/游戏名-ID-发布时间-标题名：发布时间在任务创建时从素材投影解析并落列
-- （filetransfer 模块不许读 production 表，见 architecture_test.go），租约再把 published_at
-- 带给执行器用于命名。
-- 可选列：素材无发布时间 / 查不到时为空，执行器省略该段。
ALTER TABLE file_transfer_tasks ADD COLUMN published_at DATETIME(6) NULL AFTER game_name;
