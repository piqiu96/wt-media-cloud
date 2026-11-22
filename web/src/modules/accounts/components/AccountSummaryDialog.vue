<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { invoke } from '@tauri-apps/api/core';
import { createMediaAccountClient } from '../../../shared/api/mediaAccounts.js';
import { createProfileBindingClient } from '../../../shared/api/profileBindings.js';
import { createSessionClient } from '../../../shared/api/session.js';
import { createLocalAgentService } from '../../../apps/desktop/features/local-agent/service.js';
import { executionErrorMessage } from '../../../shared/utils/executionErrors.js';
import { platformLabel } from '../../../shared/utils/platforms.js';
import { loginStatusText } from '../../../shared/utils/mediaAccountLabels.js';
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue';
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue';
import ResourceDescriptions from '../../../shared/ui/resource/ResourceDescriptions.vue';
import { feedback } from '../../../shared/ui/feedback/index.js';
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue';

const props = defineProps({ visible: Boolean, accountId: String });
const emit = defineEmits(['update:visible']);
const accounts = createMediaAccountClient(), profiles = createProfileBindingClient(), session = createSessionClient();
const account = ref(null), profile = ref(null), user = ref(null);
const loading = ref(false), opening = ref(false), error = ref('');
let sequence = 0;
const openHint = computed(() => {
  if (!account.value) return '请先读取账号信息';
  if (globalThis.window?.__WT_MEDIA_APP__ !== 'desktop' || !globalThis.window?.__TAURI_INTERNALS__) return '请在起飞桌面端打开浏览器窗口';
  if (!['operator', 'senior_operator'].includes(user.value?.role)) return '当前角色不能执行本地窗口操作';
  if (String(account.value.user_id) !== String(user.value?.id)) return '仅账号本人可打开浏览器窗口';
  if (!account.value.browser_profile_id) return '账号尚未绑定浏览器窗口';
  if (!profile.value) return '未找到可用的绑定窗口，请到社媒账号页检查';
  if (String(profile.value.user_id) !== String(user.value?.id)) return '仅窗口本人可打开浏览器窗口';
  if (profile.value.enabled === false || profile.value.local_state !== 'active' || !profile.value.bit_profile_id) return '绑定窗口不可用，请到浏览器窗口页检查';
  return '';
});
async function load() {
  const n = ++sequence;
  account.value = null; profile.value = null; user.value = null;
  loading.value = true; error.value = '';
  try {
    const [current, me] = await Promise.all([accounts.get(props.accountId), session.me()]);
    if (n !== sequence || !props.visible) return false;
    account.value = current; user.value = me;
    if (current.browser_profile_id) {
      const items = await profiles.listProfiles({ id: String(current.browser_profile_id) });
      if (n !== sequence || !props.visible) return false;
      profile.value = items.find(item => String(item.id) === String(current.browser_profile_id)) || null;
    }
    return true;
  } catch (e) {
    if (n === sequence && props.visible) error.value = e.message || '账号信息暂时无法读取，请重试';
    return false;
  } finally { if (n === sequence) loading.value = false; }
}
async function openWindow() {
  if (opening.value || loading.value || openHint.value) return;
  opening.value = true;
  try {
    // Read the current binding again: a task snapshot is not a window target.
    if (!(await load())) return;
    if (openHint.value) { error.value = openHint.value; return; }
    await createLocalAgentService({ invoke }).profileOpen(String(profile.value.bit_profile_id));
    feedback.toast.success(`已打开浏览器窗口：${profile.value.bit_name || '绑定窗口'}`);
  } catch (e) { error.value = executionErrorMessage(e); }
  finally { opening.value = false; }
}
function close() { if (!opening.value) emit('update:visible', false); }
watch(() => [props.visible, props.accountId], ([visible]) => {
  if (visible) load();
  else { sequence++; loading.value = false; account.value = null; profile.value = null; error.value = ''; }
}, { immediate: true });
onBeforeUnmount(() => { sequence++; });
</script>

<template>
  <ResourceOverlay :visible="visible" title="账号信息" :width="640" :busy="opening" form-actions @update:visible="value => { if (!value) close(); }">
    <p v-if="loading" role="status">正在读取账号信息…</p>
    <FeedbackAlert v-if="error" theme="error" :message="error"><template #operation><t-button variant="text" :disabled="opening || loading" @click="load">重新读取</t-button></template></FeedbackAlert>
    <template v-if="account">
      <ResourceSection title="账号概况">
        <ResourceDescriptions>
          <t-descriptions-item label="账号名称">{{ account.platform_name || '未识别账号' }}</t-descriptions-item>
          <t-descriptions-item label="平台">{{ platformLabel(account.platform) }}</t-descriptions-item>
          <t-descriptions-item label="真实 UID">{{ account.platform_account_id || '尚未识别' }}</t-descriptions-item>
          <t-descriptions-item label="所属用户">{{ account.user_name || '—' }}</t-descriptions-item>
          <t-descriptions-item label="账号状态">{{ account.enabled ? '启用' : '停用' }}</t-descriptions-item>
          <t-descriptions-item label="登录状态">{{ loginStatusText(account.platform_login_status) }}</t-descriptions-item>
          <t-descriptions-item label="备注" full>{{ account.remark || '—' }}</t-descriptions-item>
        </ResourceDescriptions>
      </ResourceSection>
      <ResourceSection title="当前绑定窗口">
        <ResourceDescriptions>
          <t-descriptions-item label="窗口名称">{{ profile?.bit_name || (account.browser_profile_id ? '暂不可用' : '未绑定') }}</t-descriptions-item>
          <t-descriptions-item label="窗口分组">{{ profile?.bit_group_name || '—' }}</t-descriptions-item>
        </ResourceDescriptions>
        <p v-if="!loading && openHint" class="account-window-hint">{{ openHint }}</p>
      </ResourceSection>
    </template>
    <template #footer>
      <t-button variant="outline" :disabled="opening" @click="close">关闭</t-button>
      <t-button theme="primary" :loading="opening" :disabled="loading || !!openHint" @click="openWindow">打开浏览器窗口</t-button>
    </template>
  </ResourceOverlay>
</template>

<style scoped>
.account-window-hint { color: var(--wt-text-secondary); margin: 12px 0 0; line-height: 22px; }
</style>
