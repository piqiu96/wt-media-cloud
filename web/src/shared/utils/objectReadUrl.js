const S3_ORIGIN = 'https://s3.oss.longyanyue.cn'
const PUBLIC_ORIGIN = 'https://data.bucket.oss.longyanyue.cn'
const BUCKET_PATH = '/data/'

// S3 写入/签名接口使用 path-style 的 /data/<object-key>；浏览器读取同一
// data 桶时使用公开域名，路径只保留 <object-key>，也不携带 S3 签名参数。
export function publicObjectReadUrl(address) {
  if (typeof address !== 'string' || !address.trim()) return ''
  try {
    const url = new URL(address)
    if (url.origin === S3_ORIGIN && url.pathname.startsWith(BUCKET_PATH) && url.pathname.length > BUCKET_PATH.length) {
      return `${PUBLIC_ORIGIN}/${url.pathname.slice(BUCKET_PATH.length)}`
    }
  } catch {
    // 未知格式交还原调用方处理，不拼接或猜测对象路径。
  }
  return address
}
