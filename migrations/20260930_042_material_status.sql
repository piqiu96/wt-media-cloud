-- 素材库自己的生命周期，与 video_status 是两个维度（规范 §7.2）：可以同时是「已暂停 + 可下载」。
-- NOT NULL DEFAULT 'available' —— 既有行的真实状态就是「还在提供给运营选用」，不是补一个占位。
-- 只加列，不回填改写；写路径（暂停 / 下架）尚未存在，所以读到的值今天恒为 available。
ALTER TABLE materials ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'available' AFTER video_status;

ALTER TABLE materials ADD CONSTRAINT chk_materials_status CHECK (status IN ('available', 'paused', 'delisted'));
