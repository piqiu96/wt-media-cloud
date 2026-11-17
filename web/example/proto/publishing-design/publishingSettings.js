import { platformPublishingDefaults } from './platformPublishingDefaults.js';
export const publishMethod = 'local_assisted';
export const publishMethodLabel = '浏览器辅助发布（自动提交）';
export function publishingDefaults(platform) {
  return { ...(platformPublishingDefaults[platform] || {}) };
}
export function resolvedPublishingDefaults(_db, platform, asset) {
  const settings = publishingDefaults(platform);
  const resolve = value => (value || '').replaceAll('{{标题}}', asset?.title || '').replaceAll('{{类目}}', asset?.category || '');
  const tags = resolve(settings.tags);
  return {
    tags: platform === '小红书' ? '' : tags,
    topics: platform === '小红书' ? tags : '',
    description: resolve(settings.descriptionTemplate),
    partition: settings.partition || '',
    visibility: settings.visibility || '公开',
    publishMethod,
    publishingConfigVersion: settings.version || 1,
  };
}
