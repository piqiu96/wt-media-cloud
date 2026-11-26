import { expect, it } from 'vitest';
import { initialPlatformContent, generationPatch, generationValues, parseBilibiliTags, platformCoverProfiles, publicationTitle } from './platformContent.js';
import { contentIssues, accountUnavailable, outputUnavailable } from './eligibility.js';

it('keeps Baidu title meaning and initializes only explicitly new Bilibili content', () => {
  const rule = { defaults: { partition_id: 1008, partition_name: '游戏', tags: ['游戏'], visibility: 'public' } };
  const content = initialPlatformContent('bilibili', rule);
  content.bilibili.tags.push('演示');
  expect(rule.defaults.tags).toEqual(['游戏']);
  expect(content.bilibili.statement).toBe('内容无需标注');
  expect(content.bilibili.rights_reserved).toBe(false);
  expect(publicationTitle('baidu', { description: '旧标题', title: 'foreign' })).toBe('旧标题');
  expect(publicationTitle('bilibili', { title: '新标题', description: '简介' })).toBe('新标题');
  expect(platformCoverProfiles('bilibili').map(row => [row.file, row.aspect])).toEqual([
    ['cover_file_id', [4, 3]], ['space_cover_file_id', [16, 9]],
  ]);
});

it('adopts reviewed Bilibili tags without inventing a statement or confirming content', () => {
  const content = initialPlatformContent('bilibili');
  const patch = generationPatch('bilibili', content, 'tags', '游戏\n画中画');
  expect(patch.bilibili.tags).toEqual(['游戏', '画中画']);
  expect(content.bilibili.tags).toEqual([]);
  expect(patch.bilibili.statement).toBe('内容无需标注');
  expect(generationValues('bilibili', { ...content, ...patch }).tags).toBe('游戏\n画中画');
  for (const value of ['游戏,原创', '重复\n重复', 'a\n'.repeat(11), '😀'.repeat(11)])
    expect(() => parseBilibiliTags(value)).toThrow();
});

it('checks Bilibili fields separately and counts surrogate pairs as platform UTF16 units', () => {
  const content = { title: '😀'.repeat(40), description: '', cover_file_id: '1', space_cover_file_id: '2', confirmed: true,
    bilibili: { partition_id: 1008, partition_name: '游戏', tags: ['游戏'], statement: '内容含营销信息', rights_reserved: false, visibility: 'public' } };
  const form = { platform: 'bilibili', category_id: '1', media_account_id: '17', composite_output_id: '141', content };
  expect(contentIssues(form)).toEqual([]);
  expect(contentIssues({ ...form, content: { ...content, title: content.title + 'x' } }).some(i => i.field === 'title')).toBe(true);
  expect(contentIssues({ ...form, content: { ...content, space_cover_file_id: '', portrait_cover_file_id: '9' } }).some(i => i.field === 'space')).toBe(true);
  expect(contentIssues({ ...form, content: { ...content, bilibili: { ...content.bilibili, statement: '内容为转载', source: '' } } }).some(i => i.field === 'source')).toBe(true);
});
