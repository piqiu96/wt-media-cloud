import { createSSRApp, ref } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { expect, it, vi } from 'vitest';
import BilibiliContentFields from './components/BilibiliContentFields.vue';

vi.mock('./usePublicationPlatforms.js', () => ({
  usePublicationPlatforms: () => ({ loading: ref(false), error: ref(''), ruleFor: () => ({ statement_options: ['内容无需标注', '内容含营销信息'] }) }),
}));

it('renders ordinary statement as readonly and preserves an existing historical statement', async () => {
  for (const statement of ['内容无需标注', '内容含营销信息']) {
    const content = { bilibili: { statement, tags: [] } };
    const app = createSSRApp(BilibiliContentFields, { content });
    const html = await renderToString(app);
    const field = html.split('aria-label="B站创作声明"')[1].split('</label>')[0];
    expect(field).toMatch(new RegExp(`<input[^>]*readonly[^>]*value="${statement}"`));
    expect(field).not.toContain('t-select');
    expect(content.bilibili.statement).toBe(statement);
    expect(html).toContain(statement === '内容无需标注' ? '普通发布固定使用' : '保留已保存内容的原声明');
  }
});

it('completes an unspecified editable draft through a patch and leaves disabled content untouched', async () => {
  for (const disabled of [false, true]) {
    const content = { bilibili: { statement: '', tags: ['游戏'] } };
    const patch = vi.fn();
    await renderToString(createSSRApp(BilibiliContentFields, { content, disabled, onPatch: patch }));
    if (disabled) expect(patch).not.toHaveBeenCalled();
    else expect(patch).toHaveBeenCalledExactlyOnceWith({ bilibili: { statement: '内容无需标注', tags: ['游戏'] } });
    expect(content.bilibili.statement).toBe('');
  }
});
