import { formatDateTime } from '../../shared/utils/datetime.js'

const timeZone = 'Asia/Shanghai'
const inputClock = new Intl.DateTimeFormat('en-CA', {
  timeZone, hourCycle: 'h23', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
})

export function formatPublicationTime(value) {
  return formatDateTime(value, { timeZone })
}

export function publicationScheduleInput(value) {
  if (value === null || value === undefined || value === '') return ''
  const parts = Object.fromEntries(inputClock.formatToParts(new Date(value)).map(({ type, value }) => [type, value]))
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}`
}

// The picker supplies a calendar minute without an offset. Resolve that minute
// in the business timezone rather than letting Date use the computer's timezone.
export function publicationScheduleUTC(value) {
  if (!value) return null
  const invalid = () => new Error('请选择有效的北京时间')
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) throw invalid()
  const calendar = new Date(`${value}:00Z`)
  if (!Number.isFinite(calendar.getTime()) || calendar.toISOString().slice(0, 16) !== value) throw invalid()
  let instant = calendar.getTime()
  for (let attempt = 0; attempt < 3; attempt++) {
    const displayed = publicationScheduleInput(instant)
    const difference = calendar.getTime() - Date.parse(`${displayed}:00Z`)
    if (difference === 0) return new Date(instant).toISOString()
    instant += difference
  }
  throw invalid()
}

export function publicationCreatedRangeParams(range) {
  if (!Array.isArray(range) || !range[0] || !range[1]) return {}
  const from = publicationScheduleUTC(`${range[0]}T00:00`)
  const through = publicationScheduleUTC(`${range[1]}T00:00`)
  if (from > through) throw new Error('结束日期不能早于开始日期')
  const nextDay = new Date(`${range[1]}T00:00:00Z`)
  nextDay.setUTCDate(nextDay.getUTCDate() + 1)
  return {
    created_from: from,
    created_before: publicationScheduleUTC(`${nextDay.toISOString().slice(0, 10)}T00:00`),
  }
}
