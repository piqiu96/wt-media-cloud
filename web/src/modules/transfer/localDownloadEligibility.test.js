import { describe, expect, it } from 'vitest'
import { localDownloadReason } from './localDownloadEligibility.js'
describe('Local Agent download eligibility', () => {
 it.each(['operator','senior_operator'])('permits %s to request local execution', role => expect(localDownloadReason({role})).toBe(''))
 it.each(['admin','unknown'])('explains why %s cannot start a local queue', role => expect(localDownloadReason({role})).toContain('仅对运营和高级运营开放'))
 it('blocks a new queue while session permission is unresolved', () => expect(localDownloadReason(null)).toBeTruthy())
})
