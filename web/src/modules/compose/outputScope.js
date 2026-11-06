// Drilldown is the same queried population, with its own pagination.
export function materialOutputQuery(scope, materialId, page, pageSize) {
  return { ...scope, material_id: materialId, page, page_size: pageSize }
}
