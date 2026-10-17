// 「可覆盖参数」的四件事，全部是纯函数：预填、生效值、提交载荷、提交前校验。
// 四者共用同一份「什么算填了」的判断，所以页面上看到的、发出去的、被拦下的永远是同一口径。
//
// 口径：留空 = 用策略里保存的值；填了 = 覆盖它。因此清空某一项就是回落到策略值，而不是
// 发一个空值上去（schema 的 additionalProperties:false 与类型检查都会拒绝空值）。

function isFilled(value) {
  return value !== undefined && value !== null && String(value).trim() !== ''
}

// 无字段类型信息时的原样返回：策略里存的是数字，就填回数字。
function coerce(field, raw) {
  if (field.type === 'integer' || field.type === 'number') {
    const value = Number(raw)
    return Number.isFinite(value) ? value : raw
  }
  return raw
}

// 打开抽屉/换策略时把策略里已保存的值填进输入框：这些值本来就会生效，空着反而让运营以为
// 「没设置」。策略里没有的键留空，仍由 schema 的必填规则兜底。
export function prefillOverrides(fields, strategyConfig) {
  const prefilled = {}
  const config = strategyConfig && typeof strategyConfig === 'object' ? strategyConfig : {}
  for (const field of fields || []) {
    const value = config[field.key]
    if (value === undefined || value === null) continue
    prefilled[field.key] = coerce(field, value)
  }
  return prefilled
}

// 生效值 = 策略值被填了的覆盖值顶掉。这是提交前校验与确认区共同依据的配置。
export function effectiveConfig(fields, strategyConfig, overrides) {
  const effective = {}
  const config = strategyConfig && typeof strategyConfig === 'object' ? strategyConfig : {}
  const filled = overrides && typeof overrides === 'object' ? overrides : {}
  for (const field of fields || []) {
    if (isFilled(filled[field.key])) {
      effective[field.key] = coerce(field, filled[field.key])
    } else if (config[field.key] !== undefined && config[field.key] !== null) {
      effective[field.key] = config[field.key]
    }
  }
  return effective
}

// 只发与策略值不同的键：填了等于策略值的那些没有必要发，发了也只是把同一个值再写一遍，
// 还会让确认区的「覆盖参数」列出一堆其实没改的参数。
export function buildOverridePayload(fields, strategyConfig, overrides) {
  const payload = {}
  const config = strategyConfig && typeof strategyConfig === 'object' ? strategyConfig : {}
  const filled = overrides && typeof overrides === 'object' ? overrides : {}
  for (const field of fields || []) {
    if (!isFilled(filled[field.key])) continue
    const value = coerce(field, filled[field.key])
    if (value === config[field.key]) continue
    payload[field.key] = value
  }
  return payload
}

// 跨字段规则：名字成对出现时才有意义，所以按字段名判断，而不是按模板编码。
// 错误挂到后一个字段上 —— 运营改的通常也是它。
const CROSS_FIELD_RULES = [{
  smaller: 'start_seconds',
  larger: 'end_seconds',
  message: '「结束秒数」必须大于「开始秒数」',
}]

// 返回 { 字段: 中文说明 }；空对象表示可以提交。校验的是**生效值**：策略里存着
// start 0 / end 30，运营只把 end 改成 0 同样要被拦住（这正是线上那次报错的形状）。
export function validateOverrides(fields, strategyConfig, overrides) {
  const errors = {}
  const effective = effectiveConfig(fields, strategyConfig, overrides)
  for (const field of fields || []) {
    const value = effective[field.key]
    if (value === undefined || value === null || String(value).trim() === '') {
      if (field.required) errors[field.key] = `请填写「${field.label}」`
      continue
    }
    if (field.type === 'integer' || field.type === 'number') {
      const number = Number(value)
      if (!Number.isFinite(number)) {
        errors[field.key] = `「${field.label}」必须是数字`
        continue
      }
      if (field.type === 'integer' && !Number.isInteger(number)) {
        errors[field.key] = `「${field.label}」必须是整数`
        continue
      }
      if (typeof field.minimum === 'number' && number < field.minimum) {
        errors[field.key] = `「${field.label}」不能小于 ${field.minimum}`
        continue
      }
    }
  }
  for (const rule of CROSS_FIELD_RULES) {
    const smaller = Number(effective[rule.smaller])
    const larger = Number(effective[rule.larger])
    if (!Number.isFinite(smaller) || !Number.isFinite(larger)) continue
    // 已经有一条更具体的说明（超下限、不是数字）时不再改写：那一条才是要动手改的。
    if (smaller >= larger && !errors[rule.larger]) errors[rule.larger] = rule.message
  }
  return errors
}
