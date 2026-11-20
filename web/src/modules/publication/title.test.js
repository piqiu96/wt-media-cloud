import { describe, expect, it } from 'vitest';
import { publicationTitleForOutput } from './title.js';

describe('publication source title', () => {
  const first = { id: 1, material_title: '原素材标题', name: '生成的成片名称 · 策略' };
  const second = { id: 2, material_title: '另一条素材标题' };
  it('defaults from the material title rather than the generated output name', () => {
    expect(publicationTitleForOutput('', null, first)).toBe('原素材标题');
    expect(publicationTitleForOutput('', null, { id: 3, name: '文件名称' })).toBe('');
  });
  it('updates an untouched default when replacing the output, and clears it when invalidated', () => {
    expect(publicationTitleForOutput('原素材标题', first, second)).toBe('另一条素材标题');
    expect(publicationTitleForOutput('原素材标题', first, null)).toBe('');
  });
  it('preserves a manually edited title and does not refill the same selection', () => {
    expect(publicationTitleForOutput('人工修改的标题', first, second)).toBe('人工修改的标题');
    expect(publicationTitleForOutput('', first, first)).toBe('');
  });
});
