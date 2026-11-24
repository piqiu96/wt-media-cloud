// Creation buffers are not formal records. Keep legacy records intact for compatibility.
const copy = value => JSON.parse(JSON.stringify(value));
export function batchCreationDraft(batch = null) {
  const targets = copy(batch?.targets || []);
  return {
    mode: '批量发布', step: 0, platforms: batch ? [batch.platform] : [],
    categories: targets.filter(t => t.count > 0).map(t => t.category),
    assets: [], excludedAccounts: [], rows: [], name: batch ? batch.name + '（副本）' : '',
    planId: null, batchId: null, sourceBatchId: batch?.id || null, sourcePlanId: batch?.planId || batch?.sourcePlanId || null, targets, desiredQuantity: null, sourceAssetIds: [], returnURL: null,
    creationKey: null, active: true, categoryAccountIds: {}, accountLimits: {}, accountOrder: [],
    resourcesLocked: false, savedAt: null, taskIntervalSeconds: batch?.rules?.taskIntervalSeconds ?? 0,
  };
}
export function promoteBatchDraft(db) {
  if (db.publishDraft?.active) return;
  const consumed = db.migratedBatchDraftIds ||= [];
  const legacy = db.batches.find(b => !b.preparationOnly && !b.planId && !b.firstStartedAt && b.control !== '已取消'
    && !db.tasks.some(t => t.batchId === b.id)
    && !db.plans?.some(p => p.items?.some(i => i.kind === 'batch' && i.id === b.id))
    && !consumed.includes(b.id));
  if (!legacy) return;
  const saved = legacy.draft || {};
  db.publishDraft = {
    ...batchCreationDraft(legacy), ...copy(saved), name: saved.name || legacy.name,
    rows: copy(saved.rows || legacy.candidates || []), batchId: null, planId: null,
    sourceBatchId: legacy.sourceBatchId || null, sourcePlanId: legacy.sourcePlanId || null,
    active: true, savedAt: saved.savedAt || legacy.updatedAt || legacy.created || null,
  };
  consumed.push(legacy.id);
}
export const migrateBatchCreationDrafts = promoteBatchDraft;
