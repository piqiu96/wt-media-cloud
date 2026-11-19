// A display name contains production metadata; only the source title is copy.
export function publicationTitleForOutput(current, previous, next) {
  if (previous && next && String(previous.id) === String(next.id)) return current;
  if (current.trim() && current !== (previous?.material_title || '')) return current;
  return next?.material_title || '';
}
