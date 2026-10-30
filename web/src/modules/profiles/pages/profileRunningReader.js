// 页面内共享正在进行的只读扫描；不轮询、不缓存已结束请求、不自动重试。
export function createProfileRunningReader({ scan, canRead, apply, onError, onReading }) {
  let pending = null
  let generation = 0

  function read({ notifyError = false } = {}) {
    if (!canRead()) return Promise.resolve(null)
    if (pending) {
      if (pending.generation !== generation) {
        // 等旧视图请求结束，再为返回的本人视图读取，避免并行扫描。
        return pending.promise.then(() => read({ notifyError }), () => read({ notifyError }))
      }
      pending.notifyError ||= notifyError
      return pending.promise
    }
    const request = { generation, notifyError, promise: null }
    pending = request
    onReading(true)
    request.promise = Promise.resolve().then(scan).then(snapshot => {
      if (request.generation !== generation || !canRead()) return null
      if (!snapshot || !Array.isArray(snapshot.profiles)) throw new Error('未读取到有效的窗口运行状态')
      apply(snapshot)
      return snapshot
    }).catch(error => {
      if (request.generation !== generation || !canRead()) return null
      onError(error, { notifyError: request.notifyError })
      throw error
    }).finally(() => {
      if (pending === request) {
        pending = null
        onReading(false)
      }
    })
    return request.promise
  }

  return { read, invalidate() { generation += 1 } }
}
