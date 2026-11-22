import { createSSRApp, h } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { expect, it } from 'vitest';
import FeedbackAlert from './FeedbackAlert.vue';

async function render(slots = {}, props = {}) {
  return renderToString(createSSRApp({ render: () => h(FeedbackAlert, { message: '图片读取失败，请重试', theme: 'error', ...props }, slots) }));
}
it('retains the failure reason when a legacy default slot provides recovery actions', async () => {
  const html = await render({ default: () => h('button', '重试') });
  expect(html).toContain('图片读取失败，请重试');
  expect(html).toMatch(/t-alert__operation[^]*<button>重试/);
});
it('keeps explicit operations and descriptions with the message', async () => {
  const html = await render({ operation: () => h('button', '重新读取'), description: () => '填写内容已保留' });
  expect(html).toContain('图片读取失败，请重试');
  expect(html).toContain('重新读取');
  expect(html).toContain('填写内容已保留');
});
it('allows a custom message without losing its recovery operation', async () => {
  const html = await render({ message: () => '窗口不可用', operation: () => h('button', '重新连接') });
  expect(html).toContain('窗口不可用');
  expect(html).toContain('重新连接');
});
