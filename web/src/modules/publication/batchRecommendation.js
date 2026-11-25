// Ported from example/proto/publishing-design/batchRecommendations.js.
// Optional overall quantity guides recommendations; actual category targets remain editable.
export function recommendCategoryQuantities(capacities, quantity) {
  const rows = capacities.map(({ category, capacity }) => ({
    category, capacity: Math.max(0, Math.floor(Number(capacity) || 0)), count: 0,
  }));
  const total = rows.reduce((sum, row) => sum + row.capacity, 0);
  let remaining = Number(quantity) > 0 ? Math.min(total, Math.floor(Number(quantity))) : total;
  let active = rows.filter(row => row.capacity > 0);
  while (remaining > 0 && active.length) {
    const share = Math.max(1, Math.floor(remaining / active.length));
    for (const row of active) {
      const count = Math.min(share, remaining, row.capacity - row.count);
      row.count += count;
      remaining -= count;
    }
    active = active.filter(row => row.count < row.capacity);
  }
  return rows.map(({ category, count }) => ({ category, count }));
}
