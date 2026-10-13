import { describe, expect, it } from 'vitest'
import { scheduleFormFor, schedulePayload } from './scheduleForm.js'

describe('compose schedule form', () => {
  it('keeps manual mode disabled and sends the fixed material scope', () => {
    const { payload, errors } = schedulePayload({ ...scheduleFormFor(), enabled: true })
    expect(errors).toEqual({})
    expect(payload).toMatchObject({ enabled: false, mode: 'manual', material_scope: 'creator_usages' })
  })

  it('rejects invalid daily time and limits near the fields', () => {
    const { errors } = schedulePayload({ ...scheduleFormFor(), mode: 'daily', daily_time: '25:00', per_run_limit: 101, daily_limit: 1 })
    expect(errors).toHaveProperty('daily_time')
    expect(errors).toHaveProperty('per_run_limit')
    expect(errors).toHaveProperty('daily_limit')
  })

  it('sends only the selected timing field', () => {
    const { payload, errors } = schedulePayload({ ...scheduleFormFor(), mode: 'interval', enabled: true, interval_minutes: 15 })
    expect(errors).toEqual({})
    expect(payload).toMatchObject({ enabled: true, interval_minutes: 15 })
    expect(payload).not.toHaveProperty('daily_time')
  })
})
