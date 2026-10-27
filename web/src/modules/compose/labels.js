export const TRIGGER_TYPE_OPTIONS = [
  { value: 'manual', label: '人工创建' },
  { value: 'scheduled', label: '策略自动' },
]

export function triggerLabel(value) {
  if (value === 'scheduled') return '策略自动'
  if (value === 'manual') return '人工创建'
  return '—'
}
