import { groupOverridePayload } from './taskGroupOverrides.js'
import { settingFields } from './strategySettings.js'

export function outputRemakeSeed(detail, templates = []) {
  const output = detail?.output, task = detail?.trace?.compose_task
  if (!output?.id || !task?.setting_key || !task.setting_name || !task.config_snapshot || !task.template_code || !task.template_version) return { reason: '原任务配置记录不完整，暂不能再次制作。' }
  const exactSchema = detail.trace?.strategy?.template_schema
  const template = exactSchema ? { code: task.template_code, version: task.template_version, schema: exactSchema } : templates.find((row) => row.code === task.template_code && row.version === task.template_version)
  if (!template?.schema) return { reason: '原模板版本的参数定义不可用，暂不能再次制作。' }
  if (output.actions?.remake !== true) return { reason: output.actions?.remake_reason || '当前无法再次制作。' }
  return {
    template,
    strategy: { output_settings: [{ key: task.setting_key, name: task.setting_name, platforms: [...(task.platforms || [])], copies: 1, params: { ...task.config_snapshot } }] },
    material: { id: output.material_id, title: detail.trace?.material?.title || output.material_title || '' },
    reason: '',
  }
}

export function outputRemakeBody(detail, name, seed, draft) {
  return { name: name.trim(), source_output_id: detail.output.id, remake_confirmed: true, setting_overrides: groupOverridePayload(seed.strategy.output_settings, draft, settingFields(seed.template.schema)) }
}

export async function submitOutputRemake(detail, body, { materialsClient, composeClient, submission = {} }) {
  if (!(submission.materialAccepted && submission.key === body.request_key)) {
    await materialsClient.addUsage(detail.output.material_id, { strategy_id: detail.output.compose_strategy_id, operator_id: detail.output.operator_id })
    if (submission.key === body.request_key) submission.materialAccepted = true
  }
  return composeClient.createComposeTasks(body)
}

export function remakeTaskName(name) {
  const suffix = ' · 再次制作'
  return Array.from(name || '成片').slice(0, 60 - Array.from(suffix).length).join('') + suffix
}
