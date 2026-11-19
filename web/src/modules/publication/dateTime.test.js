import { afterEach, describe, expect, it } from 'vitest'
import { formatPublicationTime, publicationScheduleInput, publicationScheduleUTC, publicationCreatedRangeParams } from './dateTime.js'

const previousTimezone = process.env.TZ
afterEach(() => {
  if (previousTimezone === undefined) delete process.env.TZ
  else process.env.TZ = previousTimezone
})

describe.each(['UTC', 'Pacific/Auckland', 'America/Los_Angeles'])('publishing business clock in %s', (timezone) => {
  it('displays and edits the same instant in Beijing time', () => {
    process.env.TZ = timezone
    expect(formatPublicationTime('2026-11-17T16:48:00Z')).toBe('2026/11/18 00:48')
    expect(publicationScheduleInput('2026-11-17T16:48:00Z')).toBe('2026-11-18T00:48')
    expect(publicationScheduleUTC('2026-11-18T00:48')).toBe('2026-11-17T16:48:00.000Z')
  })

  it('queries whole Beijing calendar days including a year boundary', () => {
    process.env.TZ = timezone
    expect(publicationCreatedRangeParams(['2026-12-31', '2027-01-01'])).toEqual({
      created_from: '2026-12-30T16:00:00.000Z',
      created_before: '2027-01-01T16:00:00.000Z',
    })
  })
})

describe('publication time validation', () => {
  it('resolves a calendar minute whose initial UTC candidate is the Unix epoch', () => {
    expect(publicationScheduleInput(0)).toBe('1970-01-01T08:00')
    expect(publicationScheduleUTC('1970-01-01T00:00')).toBe('1969-12-31T16:00:00.000Z')
  })

  it('keeps unset scheduling immediate and a cleared range unfiltered', () => {
    expect(publicationScheduleUTC('')).toBeNull()
    expect(publicationScheduleInput(null)).toBe('')
    expect(formatPublicationTime(null)).toBe('-')
    expect(publicationCreatedRangeParams([])).toEqual({})
    expect(publicationCreatedRangeParams(['2026-11-18', ''])).toEqual({})
  })

  it('rejects normalized invalid dates and malformed scheduling without sending an unintended instant', () => {
    for (const value of ['2026-02-30T12:30', '2026-11-18T24:30', '2026-11-18T09:00Z', 'not a date']) {
      expect(() => publicationScheduleUTC(value)).toThrow()
    }
    expect(() => publicationCreatedRangeParams(['2026-11-19', '2026-11-18'])).toThrow()
    expect(() => publicationCreatedRangeParams(['2026-02-30', '2026-03-01'])).toThrow()
  })

  it('uses the next business midnight even across a historic Shanghai daylight saving change', () => {
    expect(publicationCreatedRangeParams(['1991-04-14', '1991-04-14'])).toEqual({
      created_from: '1991-04-13T16:00:00.000Z',
      created_before: '1991-04-14T15:00:00.000Z',
    })
  })
})
