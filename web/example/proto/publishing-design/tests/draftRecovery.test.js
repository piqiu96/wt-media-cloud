import { describe, it, expect, vi, beforeEach } from 'vitest';
const plugin = vi.hoisted(() => ({ config: null, destroy: vi.fn() }));
vi.mock('tdesign-vue-next', () => ({ DialogPlugin: { confirm: vi.fn(config => { plugin.config = config; return { destroy: plugin.destroy }; }) } }));
import { chooseDraft, draftSavedMessage } from '../draftRecovery.js';
beforeEach(() => { plugin.destroy.mockClear(); });
describe('single creation draft prompt', () => {
  it('continues an existing draft without conflating close or restart', async () => {
    const result = chooseDraft({ label: '发布任务', savedAt: '2026-11-14T02:32:00Z' });
    expect(plugin.config.header).toBe('发现未完成的发布任务配置');
    expect(plugin.config.confirmBtn.content).toBe('继续编辑');
    expect(plugin.config.cancelBtn.content).toBe('重新开始');
    plugin.config.onConfirm(); plugin.config.onClose({ trigger: 'confirm' });
    await expect(result).resolves.toBe('continue'); expect(plugin.destroy).toHaveBeenCalledTimes(1);
  });
  it('returns restart only when explicitly chosen', async () => {
    const result = chooseDraft(); plugin.config.onCancel(); plugin.config.onClose({ trigger: 'cancel' });
    await expect(result).resolves.toBe('restart');
  });
  it('closing keeps the user on their source page', async () => {
    const result = chooseDraft(); plugin.config.onClose({ trigger: 'close-btn' });
    await expect(result).resolves.toBeNull();
  });
  it('does not invent a save time for legacy drafts', () => {
    expect(draftSavedMessage(null)).not.toContain('上次保存于');
    expect(draftSavedMessage('bad')).not.toContain('Invalid Date');
    expect(draftSavedMessage('2026-11-14T02:32:00Z')).toContain('上次保存于');
  });
});
