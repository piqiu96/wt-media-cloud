import { createSSRApp, reactive } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';

const api = vi.hoisted(() => ({ get: vi.fn(), listProfiles: vi.fn(), me: vi.fn(), invoke: vi.fn() }));
vi.mock('../../../shared/api/mediaAccounts.js', () => ({ createMediaAccountClient: () => ({ get: api.get }) }));
vi.mock('../../../shared/api/profileBindings.js', () => ({ createProfileBindingClient: () => ({ listProfiles: api.listProfiles }) }));
vi.mock('../../../shared/api/session.js', () => ({ createSessionClient: () => ({ me: api.me }) }));
vi.mock('@tauri-apps/api/core', () => ({ invoke: api.invoke }));
import AccountSummaryDialog from './AccountSummaryDialog.vue';

const account = { id: '9', user_id: '1', platform_name: '运营账号', browser_profile_id: '8' };
const profile = { id: '8', user_id: '1', bit_profile_id: 'bit-8', bit_name: '运营窗口', enabled: true, local_state: 'active' };
let state, props;
beforeEach(async () => {
  vi.resetAllMocks();
  vi.stubGlobal('window', { __WT_MEDIA_APP__: 'desktop', __TAURI_INTERNALS__: {} });
  api.get.mockResolvedValue({ ...account });
  api.me.mockResolvedValue({ id: '1', role: 'operator' });
  api.listProfiles.mockResolvedValue([{ ...profile }]);
  api.invoke.mockResolvedValue({});
  props = reactive({ visible: false, accountId: '9' });
  await renderToString(createSSRApp({ setup(_, context) {
    state = AccountSummaryDialog.setup(props, context);
    return () => null;
  } }));
  props.visible = true;
  await state.load();
});
afterEach(() => vi.unstubAllGlobals());

it('reads only the bound profile and opens it only after an explicit action', async () => {
  expect(api.listProfiles).toHaveBeenCalledWith({ id: '8' });
  expect(api.invoke).not.toHaveBeenCalled();
  await state.openWindow();
  expect(api.invoke).toHaveBeenCalledWith('local_agent_profile_open', { args: { bit_profile_id: 'bit-8' } });
  expect(state.notice.value).toBe('已打开浏览器窗口：运营窗口');
});

it('rechecks the current binding before opening instead of using the displayed snapshot', async () => {
  api.get.mockResolvedValue({ ...account, browser_profile_id: '10' });
  api.listProfiles.mockResolvedValue([{ ...profile, id: '10', bit_profile_id: 'bit-10' }]);
  await state.openWindow();
  expect(api.invoke).toHaveBeenCalledWith('local_agent_profile_open', { args: { bit_profile_id: 'bit-10' } });
});

it.each([
  ['web', () => { window.__WT_MEDIA_APP__ = 'cloud'; }, '桌面端'],
  ['other owner', () => { state.account.value.user_id = '2'; }, '账号本人'],
  ['admin', () => { state.user.value.role = 'admin'; }, '当前角色'],
  ['unbound', () => { state.account.value.browser_profile_id = null; }, '尚未绑定'],
  ['missing profile', () => { state.profile.value = null; }, '未找到'],
  ['other profile owner', () => { state.profile.value.user_id = '2'; }, '窗口本人'],
  ['disabled profile', () => { state.profile.value.enabled = false; }, '不可用'],
  ['inactive profile', () => { state.profile.value.local_state = 'released'; }, '不可用'],
])('explains %s without invoking a local window action', async (_, change, hint) => {
  change();
  expect(state.openHint.value).toContain(hint);
  await state.openWindow();
  expect(api.invoke).not.toHaveBeenCalled();
});

it('stops opening if ownership changes during revalidation', async () => {
  api.get.mockResolvedValue({ ...account, user_id: '2' });
  await state.openWindow();
  expect(api.invoke).not.toHaveBeenCalled();
  expect(state.error.value).toContain('账号本人');
});

it('recovers from a read failure and does not retain stale account information', async () => {
  api.get.mockRejectedValueOnce(new Error('账号已删除或无权查看'));
  await state.load();
  expect(state.error.value).toBe('账号已删除或无权查看');
  expect(state.account.value).toBeNull();
  await state.load();
  expect(state.error.value).toBe('');
  expect(state.account.value.id).toBe('9');
});

it('ignores an earlier response after switching accounts', async () => {
  let finish;
  api.get.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const earlier = state.load();
  props.accountId = '11';
  api.get.mockResolvedValue({ ...account, id: '11' });
  await state.load();
  finish(account);
  await earlier;
  expect(state.account.value.id).toBe('11');
});

it('prevents overlapping opens and reports a native failure without claiming success', async () => {
  let fail;
  api.invoke.mockImplementation(() => new Promise((_, reject) => { fail = reject; }));
  const opening = state.openWindow();
  await vi.waitFor(() => expect(api.invoke).toHaveBeenCalledOnce());
  await state.openWindow();
  expect(api.invoke).toHaveBeenCalledOnce();
  fail(new Error('浏览器连接失败'));
  await opening;
  expect(state.opening.value).toBe(false);
  expect(state.error.value).toContain('浏览器连接失败');
  expect(state.notice.value).toBe('');
});
