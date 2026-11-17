// Lossless preview-only transport: large image strings are sent once even when
// referenced by multiple tasks, historical snapshots or JSON signature strings.
const format = 'wt-media-snapshot-strings-v1';
export function packSnapshot(value) {
  const strings = [], indices = new Map();
  function pack(item) {
    if (typeof item === 'string' && item.length > 2048) {
      if (/^[\[{]/.test(item)) {
        try {
          const parsed = JSON.parse(item);
          if (JSON.stringify(parsed) === item) return [3, pack(parsed)];
        } catch { /* Non-JSON text is retained verbatim. */ }
      }
      if (!indices.has(item)) { indices.set(item, strings.length); strings.push(item); }
      return [0, indices.get(item)];
    }
    if (Array.isArray(item)) return [1, item.map(pack)];
    if (item && typeof item === 'object') return [2, Object.entries(item).map(([key, val]) => [key, pack(val)])];
    return item;
  }
  return {format, strings, value: pack(value)};
}
export function unpackSnapshot(envelope, {maxBytes = 256 * 1024 * 1024} = {}) {
  if (envelope?.format !== format) return envelope;
  if (!Array.isArray(envelope.strings) || envelope.strings.some(s => typeof s !== 'string')) throw Error('保存内容格式无效');
  const encoder = new TextEncoder(), costs = new Map();
  let bytes = 0;
  function charge(size) { bytes += size; if (bytes > maxBytes) throw Error('保存内容超过预览展开上限'); }
  function string(item) {
    if (!costs.has(item)) costs.set(item, encoder.encode(JSON.stringify(item)).length);
    charge(costs.get(item));
    return item;
  }
  function unpack(item, depth = 0) {
    if (depth > 200) throw Error('保存内容层级过深');
    if (typeof item === 'string') return string(item);
    if (!Array.isArray(item)) {
      if (item !== null && !['number', 'boolean'].includes(typeof item)) throw Error('保存内容格式无效');
      charge(JSON.stringify(item).length); return item;
    }
    if (item.length !== 2) throw Error('保存内容格式无效');
    const [tag, value] = item;
    if (tag === 0 && Number.isInteger(value) && value >= 0 && value < envelope.strings.length) return string(envelope.strings[value]);
    if (tag === 1 && Array.isArray(value)) { charge(2 + value.length); return value.map(v => unpack(v, depth + 1)); }
    if (tag === 2 && Array.isArray(value)) {
      charge(2 + value.length * 2);
      return Object.fromEntries(value.map(entry => {
        if (!Array.isArray(entry) || entry.length !== 2 || typeof entry[0] !== 'string') throw Error('保存内容格式无效');
        string(entry[0]); return [entry[0], unpack(entry[1], depth + 1)];
      }));
    }
    if (tag === 3) {
      const before = bytes, result = JSON.stringify(unpack(value, depth + 1));
      bytes = before; return string(result);
    }
    throw Error('保存内容格式无效');
  }
  return unpack(envelope.value);
}
