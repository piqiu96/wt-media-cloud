export const TRIGGER_TYPE_OPTIONS = [
  { value: 'manual', label: '人工创建' },
  { value: 'immediate', label: '立即执行' },
  { value: 'scheduled', label: '策略自动' },
]

export function triggerLabel(value) {
  if (value === 'scheduled') return '策略自动'
  if (value === 'immediate') return '立即执行'
  if (value === 'manual') return '人工创建'
  return '—'
}
