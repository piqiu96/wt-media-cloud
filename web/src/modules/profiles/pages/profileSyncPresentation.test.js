import { describe, expect, it } from 'vitest'
import { historicalMissingProfilesFromSync, missingProfilesFromScan, pendingProfilesFromSync, profileChangedValues, profileTeamName, windowStateMatches } from './profileSyncPresentation.js'

describe('sync preview missing windows', () => {
  it('shows Cloud mirror rows that cannot appear in the fresh BitBrowser snapshot', () => {
    const scan = {
      profiles: [{ bit_profile_id: 'still-here', bit_name: '现有窗口' }],
      diff: [{ kind: 'missing', profile_id: '12', bit_profile_id: 'gone' }],
    }
    expect(missingProfilesFromScan(scan, [{ id: 12, bit_profile_id: 'gone', bit_name: '已缺失的窗口' }]))
      .toEqual([{ id: 12, bit_profile_id: 'gone', bit_name: '已缺失的窗口' }])
  })

  it('retains a visible entry when the Cloud list was not available', () => {
    expect(missingProfilesFromScan({ diff: [{ kind: 'missing', profile_id: '12', bit_profile_id: 'gone' }] }, []))
      .toEqual([{ id: '12', bit_profile_id: 'gone', bit_name: '已缺失窗口' }])
  })
})

describe('sync result pending windows', () => {
  it('keeps historical missing windows separately without counting a newly missing row twice', () => {
    const scan = { diff: [{ kind: 'missing', profile_id: '22', bit_profile_id: 'new-missing' }] }
    const profiles = [
      { id: 20, bit_profile_id: 'old-missing', local_state: 'local_missing' },
      { id: 22, bit_profile_id: 'new-missing', local_state: 'local_missing' },
      { id: 23, bit_profile_id: 'found', local_state: 'active' },
    ]
    expect(historicalMissingProfilesFromSync(scan, profiles).map(profile => profile.id)).toEqual([20])
  })
  it('does not repeat historical missing windows when a rescan finds no changes', () => {
    const oldMissing = { id: 20, bit_profile_id: 'old-missing', local_state: 'local_missing' }
    expect(pendingProfilesFromSync({ diff: [] }, [oldMissing], profile => profile.local_state === 'local_missing')).toEqual([])
  })

  it('shows only issues affected by this sync and excludes previously confirmed missing windows', () => {
    const scan = { diff: [
      { kind: 'changed', profile_id: '21', bit_profile_id: 'changed' },
      { kind: 'missing', profile_id: '22', bit_profile_id: 'newly-missing' },
    ] }
    const profiles = [
      { id: 20, bit_profile_id: 'old-missing', local_state: 'local_missing' },
      { id: 21, bit_profile_id: 'changed', issue: '代理已到期' },
      { id: 22, bit_profile_id: 'newly-missing', local_state: 'local_missing' },
      { id: 23, bit_profile_id: 'unchanged', issue: '代理未检测' },
    ]
    expect(pendingProfilesFromSync(scan, profiles, profile => Boolean(profile.issue) || profile.local_state === 'local_missing')
      .map(profile => profile.id)).toEqual([21, 22])
  })
})

describe('sync preview changed values', () => {
  it('shows before and after facts, including cleared values', () => {
    const scan = {
      profiles: [{ bit_profile_id: 'one', bit_name: '新名称', bit_remark: '' }],
      diff: [{ kind: 'changed', bit_profile_id: 'one', fields: ['bit_name', 'bit_remark'] }],
    }
    expect(profileChangedValues(scan, [{ bit_profile_id: 'one', bit_name: '旧名称', bit_remark: '旧备注' }], 'one'))
      .toEqual([
        { field: 'bit_name', label: '窗口名称', before: '旧名称', after: '新名称' },
        { field: 'bit_remark', label: '比特备注', before: '旧备注', after: '未填写' },
      ])
  })
})

describe('window list presentation', () => {
  it('shows found and missing by default, but never silently includes archived records', () => {
    const rows = [{ local_state: 'active' }, { local_state: 'local_missing' }, { local_state: 'archived' }]
    expect(rows.filter(row => windowStateMatches(row)).map(row => row.local_state)).toEqual(['active', 'local_missing'])
    expect(rows.filter(row => windowStateMatches(row, 'found')).map(row => row.local_state)).toEqual(['active'])
    expect(rows.filter(row => windowStateMatches(row, 'missing')).map(row => row.local_state)).toEqual(['local_missing'])
  })
  it('shows the operator real team name without an admin roster', () => {
    const profile = { user_id: 7, team_id: 3 }
    expect(profileTeamName(profile, { id: 7, team_id: 3, team_name: '华东运营' }, [], 7)).toBe('华东运营')
  })

  it('uses the window recorded team when its owner moved to another team', () => {
    const profile = { user_id: 7, team_id: 3 }
    const owner = [{ id: 7, team_id: 9, team_name: '新团队' }]
    expect(profileTeamName(profile, { id: 7, team_id: 9, team_name: '新团队' }, owner, 7, [{ id: 3, name: '原运营团队' }])).toBe('原运营团队')
  })
})
