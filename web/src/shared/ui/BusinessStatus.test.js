import { createSSRApp } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { expect, it } from 'vitest';
import BusinessStatus from './BusinessStatus.vue';
import { publicationStatuses } from '../../modules/publication/status.js';

const tones = {
  pending: 'neutral', queued: 'neutral', running: 'info',
  awaiting_human: 'warning', uncertain: 'warning', reviewing: 'warning',
  failed: 'danger', published: 'success', cancelled: 'neutral', abandoned: 'neutral',
};
for (const [status, label] of Object.entries(publicationStatuses)) {
  it(`renders ${status} as a read-only ${tones[status]} label`, async () => {
    const html = await renderToString(createSSRApp(BusinessStatus, { status, label }));
    expect(html).toContain(`is-${tones[status]}`);
    expect(html).toContain(label);
    expect(html).not.toMatch(/<button|role="button"|tabindex/);
  });
}
it('keeps an unknown state neutral without presenting it as success', async () => {
  const html = await renderToString(createSSRApp(BusinessStatus, { status: 'new_status', label: '新状态' }));
  expect(html).toContain('is-neutral');
  expect(html).toContain('新状态');
});

it.each([['pending','neutral'],['running','info'],['success','success'],['partial_success','warning'],['interrupted','warning'],['failed','danger'],['cancelled','neutral']])('maps shared task %s to %s', async (status, tone) => {
  const html = await renderToString(createSSRApp(BusinessStatus, { status }));
  expect(html).toContain(`is-${tone}`);
});
