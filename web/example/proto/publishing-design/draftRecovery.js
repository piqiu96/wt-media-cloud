import { DialogPlugin } from 'tdesign-vue-next';

export function draftSavedMessage(savedAt) {
  const date = new Date(savedAt);
  return savedAt && !Number.isNaN(date.getTime())
    ? `上次保存于 ${date.toLocaleString('zh-CN', { month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })}，可以继续编辑或重新开始。`
    : '可以继续编辑上次配置，或重新开始。';
}
// Closing is distinct from choosing either action: it must preserve the draft.
export function chooseDraft({ label = '发布', savedAt } = {}) {
  return new Promise(resolve => {
    let settled = false;
    let dialog;
    const finish = value => {
      if (settled) return;
      settled = true;
      dialog?.destroy();
      resolve(value);
    };
    dialog = DialogPlugin.confirm({
      header: `发现未完成的${label}配置`,
      body: draftSavedMessage(savedAt),
      width: 'min(calc(100vw - 32px), 560px)',
      confirmBtn: { content: '继续编辑', theme: 'primary' },
      cancelBtn: { content: '重新开始', theme: 'default', variant: 'outline' },
      closeOnOverlayClick: false,
      onConfirm: () => finish('continue'),
      onCancel: () => finish('restart'),
      onClose: context => { if (context?.trigger !== 'cancel') finish(null); },
    });
  });
}
