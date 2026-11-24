import { reactive } from "vue";
import { createPublicationBatchClient, createPublicationClient } from "../../shared/api/publication.js";
import { createMediaAccountClient } from "../../shared/api/mediaAccounts.js";
import { outputUnavailable } from "./eligibility.js";

export const batchOutputID = row => String(row.composite_output_id ?? row.id);
export const batchOutputName = row => row?.name || row?.file_name || row?.material_title || "成片名称暂不可用";
export const batchAccountName = row => row?.name || row?.platform_name || row?.platform_account_id || "账号名称暂不可用";

export function batchOutputProblem(row, category, teamId) {
  if (typeof row.occupied !== "boolean") return "占用状态未知";
  return outputUnavailable(row, category || row.category_id, teamId ? { team_id: teamId } : null);
}

export function useBatchResourceCatalog() {
  const accounts = reactive(new Map());
  const outputs = reactive(new Map());
  const batch = createPublicationBatchClient();
  const publication = createPublicationClient();
  const media = createMediaAccountClient();

  function remember(kind, row) {
    if (kind === "account") accounts.set(String(row.id), row);
    else outputs.set(batchOutputID(row), row);
  }
  async function hydrate(form, { owner, platform, readOnly = false }) {
    const accountIds = new Set([
      ...form.accounts.map(row => row.media_account_id),
      ...form.candidates.map(row => row.media_account_id),
    ]);
    const outputIds = [...new Set([
      ...form.output_ids, ...form.candidates.map(row => row.composite_output_id),
    ])];
    const queries = [];
    for (let i = 0; i < outputIds.length; i += 100) {
      queries.push(publication.outputs({ platform, output_ids: outputIds.slice(i, i + 100).join(","), page: 1, page_size: 100 })
        .then(result => (result.items || []).forEach(row => remember("output", row))));
    }
    if (accountIds.size) {
      if (readOnly) {
        queries.push(media.list({ platform, user_id: owner }).then(rows =>
          rows.filter(row => accountIds.has(String(row.id))).forEach(row => remember("account", row))));
      } else {
        queries.push((async () => {
          for (let page = 1; ; page++) {
            const result = await batch.accounts({ platform, page, page_size: 100 });
            (result.items || []).forEach(row => remember("account", row));
            if (page * 100 >= result.total || !result.items?.length) return;
          }
        })());
      }
    }
    const results = await Promise.allSettled(queries);
    for (const result of results) {
      if (result.status === "rejected") throw result.reason;
    }
  }

  return { accounts, outputs, remember, hydrate };
}
