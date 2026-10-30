import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'

describe('discovery team filter reset', () => {
  for (const page of ['DiscoveryStrategiesPage', 'CrawlTasksPage']) {
    it(`${page}: clears the team before requesting the reset list`, () => {
      const source = readFileSync(new URL(`./${page}.vue`, import.meta.url), 'utf8')
      const command = source.match(/function resetFilters\([^]*?\n}/)[0]
      const filters = { value: { search: 'old', status: 'failed' } }, teamFilter = { value: 17 }
      const load = vi.fn(() => { expect(teamFilter.value).toBe('') })
      new Function('filters', 'teamFilter', 'load', `${command}; resetFilters()`)(filters, teamFilter, load)
      expect(load).toHaveBeenCalledOnce()
      expect(filters.value.search).toBe('')
      expect(filters.value.status).toBe('')
    })
  }
})
