import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'

const options = { placement: 'top-right', duration: 4000, closeBtn: true }

function toast(theme, message) {
  const config = message && typeof message === 'object'
    ? { ...options, ...message }
    : { ...options, content: message }
  return MessagePlugin[theme](config)
}

function confirmWarning({ title, message, confirmText = '继续', danger = false }) {
  return new Promise((resolve) => {
    let settled = false
    function finish(answer) {
      if (settled) return
      settled = true
      dialog.destroy()
      resolve(answer)
    }
    const dialog = DialogPlugin.confirm({
      theme: 'warning',
      header: title,
      body: message,
      confirmBtn: { content: confirmText, theme: danger ? 'danger' : 'primary' },
      cancelBtn: { content: '取消' },
      closeOnOverlayClick: false,
      onConfirm: () => finish(true),
      onCancel: () => finish(false),
      onClose: () => finish(false),
    })
  })
}

export const feedback = {
  confirmWarning,
  toast: {
    success(message) { return toast('success', message) },
    error(message) { return toast('error', message) },
    info(message) { return toast('info', message) },
  },
}
