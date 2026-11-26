import { computed, onMounted, ref } from 'vue';
import { createPublicationClient } from '../../shared/api/publication.js';

export function usePublicationPlatforms() {
  const rules = ref([]), loading = ref(false), error = ref('');
  const client = createPublicationClient();
  const options = computed(() => rules.value.filter(rule => rule.enabled).map(rule => ({ value: rule.platform, label: rule.name })));
  const ruleFor = platform => rules.value.find(rule => rule.platform === platform);
  async function reload() {
    if (loading.value) return;
    loading.value = true;
    error.value = '';
    try {
      const result = await client.platformRules();
      if (!result?.revision || !Array.isArray(result.platforms)) throw new Error('平台规则返回不完整');
      rules.value = result.platforms;
    } catch (failure) {
      error.value = failure.message || '平台规则读取失败，请重试';
    } finally { loading.value = false; }
  }
  onMounted(reload);
  return { rules, options, ruleFor, loading, error, reload };
}
