# Content Mining Domain Flow Plan

- Date: 2026-09-22
- Design: `docs/superpowers/specs/2026-09-22-content-mining-domain-flow-design.md`

## Status

- [x] Complete

## Steps

1. Backend: list ordering to ID DESC（策略/任务/内容池）。
2. Backend: 内容池状态变更审计人字段迁移（`source_contents.updated_by`）与写入。
3. Frontend: 任务来源模型（策略/人工/导入/重试）+ 任务命名规范。
4. Frontend: 策略操作「记录」→「任务」。
5. Frontend: 任务详情来源策略可点击 → Drawer 展示策略信息 + 「查看策略」跳转定位。
6. Frontend: 策略/任务/内容池详情暴露创建时间/修改时间/操作人（+内容池审核时间/审核人）。
7. 更新静态测试，跑 go test/vet、npm test/build。
8. 提交并重启 Cloud API + Web。
