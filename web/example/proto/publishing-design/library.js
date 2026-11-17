export function imageReferences(db, im) {
  return [
    ...db.templates
      .filter((t) => t.imageId === im.id || t.document?.image === im.src || t.draft?.document?.image === im.src)
      .map((t) => ({ kind: "模板", id: t.id, name: t.name })),
    ...db.tasks
      .filter(
        (t) =>
          t.imageId === im.id ||
          t.coverSource === im.src ||
          t.cover === im.src ||
          t.templateDocument?.image === im.src ||
          Object.values(t.templateDocuments || {}).some(
            (d) => d.image === im.src,
          ) ||
          Object.values(t.coverVariants || {}).includes(im.src),
      )
      .map((t) => ({ kind: "正式任务", id: t.id, name: t.title })),
  ];
}
export const templateReferences = (db, t) =>
  db.tasks.filter(
    (r) =>
      r.templateId === t.id ||
      Object.values(r.templateSnapshots || {}).some((s) => s.id === t.id),
  );
export function deleteImage(db, im) {
  if (imageReferences(db, im).length)
    throw Error("图片已被模板或正式任务引用，只能停用");
  db.images = db.images.filter((i) => i.id !== im.id);
}
export function chooseBackground(db, category, rng = Math.random) {
  const rows = db.images.filter((i) => i.enabled && i.uses.includes("底图"));
  const exact = rows.filter((i) => i.category === category);
  const pool = exact.length ? exact : rows.filter((i) => i.category === "通用");
  return pool.length ? pool[Math.floor(rng() * pool.length)] : null;
}
