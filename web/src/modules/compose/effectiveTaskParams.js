export function changedTaskParams(group, overrides = {}, fields = []) {
  return fields.filter(field => field.editable === true && field.key !== 'guide_video_url'
    && Object.hasOwn(overrides, field.key) && overrides[field.key] !== group.params[field.key])
    .map(field => ({ key:field.key, label:field.label, value:overrides[field.key] }))
}
