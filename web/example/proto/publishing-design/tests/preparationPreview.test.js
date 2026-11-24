import { expect, it } from "vitest";
import { preparationRecord, previewExecutionSettings } from "../preparationPreview.js";
import { batchActions, formalBatch, rowsFor } from "../batchLifecycle.js";

it("saves incomplete candidates separately from tasks and preserves the target when edited", () => {
  const actor = { id: "lin", name: "林予安" };
  const state = {
    name: "演示批次", platforms: ["百家号"], categories: ["三角洲行动"],
    targets: [{ category: "三角洲行动", count: 60 }], rows: [{ key: "first" }, { key: "second" }],
    executionSettings: { ...previewExecutionSettings(), exceptionLimit: 3 },
  };
  const record = preparationRecord(state, { id: 23, actor });
  const database = { batches: [record], tasks: [] };
  expect(formalBatch(database, record)).toBe(true);
  expect(rowsFor(database, record)).toEqual([]);
  expect(batchActions(database, record)).toEqual(["configure", "detail"]);
  state.rows = [state.rows[0]];
  const updated = preparationRecord(state, { id: record.id, actor, existing: record });
  expect(updated.candidates).toEqual([{ key: "first" }]);
  expect(updated.targets[0].count).toBe(60);
  expect(updated.executionSettings).toEqual(state.executionSettings);
});
