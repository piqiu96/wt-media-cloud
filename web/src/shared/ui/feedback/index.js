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
      width: 'min(calc(100vw - 32px), 480px)',
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

// 单按钮告知，用于「操作已完成，结果是什么」这类必须被看见、但不改变流程的消息。
// 返回 Promise 只是让调用方能等到用户点掉；业务流程不依赖它的结果。
function notice({ title, message, confirmText = '知道了' }) {
  return new Promise((resolve) => {
    let settled = false
    function finish() {
      if (settled) return
      settled = true
      dialog.destroy()
      resolve(true)
    }
    const dialog = DialogPlugin.alert({
      header: title,
      width: 'min(calc(100vw - 32px), 480px)',
      body: message,
      confirmBtn: { content: confirmText, theme: 'primary' },
      closeOnOverlayClick: false,
      onConfirm: finish,
      onClose: finish,
    })
  })
}

export const feedback = {
  confirmWarning,
  notice,
  toast: {
    success(message) { return toast('success', message) },
    error(message) { return toast('error', message) },
    info(message) { return toast('info', message) },
  },
}
