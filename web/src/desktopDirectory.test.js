import { describe, it, expect, vi } from 'vitest'
import { createLocalSettingsService, normalizeSettingsView } from './apps/desktop/features/local-settings/service.js'

describe('桌面目录桥接', () => {
 it('reads the persisted directory registration without inventing data', () => {
  expect(normalizeSettingsView({ save_dir: '/new', file: '/data/settings.toml', known_save_dirs: ['/new','/old'] }).knownSaveDirs).toEqual(['/new','/old'])
 })
 it('cancelled native picker returns null without storing or pushing', async () => {
  const invoke=vi.fn().mockResolvedValue(null), service=createLocalSettingsService({invoke})
  expect(await service.pickSaveDirectory()).toBeNull()
  expect(invoke.mock.calls).toEqual([['local_pick_save_directory']])
 })
 it('removes only the directory registration and opens through controlled commands', async () => {
  const invoke=vi.fn().mockResolvedValue({save_dir:'/new',file:'/data/settings.toml',known_save_dirs:['/new']})
  const service=createLocalSettingsService({invoke})
  expect((await service.removeSearchDirectory('/old')).knownSaveDirs).toEqual(['/new'])
  expect(invoke).toHaveBeenLastCalledWith('local_remove_search_directory',{directory:'/old'})
  invoke.mockResolvedValue('/new')
  expect(await service.openSaveDirectory('/new')).toBe('/new')
  expect(invoke).toHaveBeenLastCalledWith('local_open_save_directory',{directory:'/new'})
 })
})
