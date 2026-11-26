// The platform API owns defaults and availability. These helpers preserve the
// existing Baidu content meaning and project explicit Bilibili input only.
export const isBilibili = platform => platform === 'bilibili';
export const publicationTitle = (platform, content) => isBilibili(platform) ? content?.title || '' : content?.description || '';
export const titleField = platform => isBilibili(platform) ? 'title' : 'description';
export function publicationContentInput(platform, content) {
  if (!isBilibili(platform)) return { ...content };
  const allowed = ['title','description','bilibili','cover_file_id','space_cover_file_id','cover_render','space_cover_render','confirmed'];
  return Object.fromEntries(Object.entries(content).filter(([field,value]) => allowed.includes(field) && (!field.endsWith('_file_id') || value && value !== '0')));
}
export function initialPlatformContent(platform, rule) {
  if (!isBilibili(platform)) return { description: '', topics: '', declaration: '', confirmed: false };
  const defaults = rule?.defaults || {};
  return { title: '', description: '', bilibili: {
    partition_id: defaults.partition_id || 0, partition_name: defaults.partition_name || '',
    tags: [...(defaults.tags || [])], visibility: defaults.visibility || 'public',
    statement: defaults.statement || '内容无需标注', rights_reserved: false, source: '',
  }, confirmed: false };
}
export const platformGenerationFields = platform => isBilibili(platform)
  ? { title: '发布标题', description: '简介', tags: '标签', cover_title: '封面短标题' }
  : { description: '发布标题', topics: '话题', cover_title: '封面短标题' };
export function parseBilibiliTags(value) {
  const tags = String(value || '').split('\n').map(tag => tag.trim()).filter(Boolean);
  if (!tags.length || tags.length > 10 || tags.some(tag => tag.length > 20 || /[,，\r]/.test(tag)) || new Set(tags).size !== tags.length)
    throw new Error('请填写1至10个不重复的标签，每行一个，每个不超过20字');
  return tags;
}
export function generationValues(platform, content, coverTitle = '') {
  if (!isBilibili(platform)) return { description: content?.description || '', topics: content?.topics || '', cover_title: coverTitle };
  return { title: content?.title || '', description: content?.description || '', tags: (content?.bilibili?.tags || []).join('\n'), cover_title: coverTitle };
}
export function generationPatch(platform, content, field, value) {
  if (isBilibili(platform) && field === 'tags') return { bilibili: { ...content.bilibili, tags: parseBilibiliTags(value) } };
  return { [field]: value };
}
export function platformCoverProfiles(platform) {
  return isBilibili(platform) ? [
    { key: 'horizontal', label: '首页推荐封面 · 4:3', aspect: [4, 3], file: 'cover_file_id', render: 'cover_render' },
    { key: 'space', label: '个人空间封面 · 16:9', aspect: [16, 9], file: 'space_cover_file_id', render: 'space_cover_render' },
  ] : [
    { key: 'horizontal', label: '横版封面 · 16:9', aspect: [16, 9], file: 'cover_file_id', render: 'cover_render' },
    { key: 'portrait', label: '竖版封面 · 3:4', aspect: [3, 4], file: 'portrait_cover_file_id', render: 'portrait_cover_render' },
  ];
}
