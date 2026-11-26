import { describe, expect, it, vi } from "vitest";
import * as batch from "./batchPreparation.js";

const pair = (account, output) => ({
  category_id: "2",
  media_account_id: account,
  composite_output_id: output,
});
const candidate = { ...pair("3", "4"), row_key: "row-one", order: 0,
  input_revision: "9007199254740993", field_revisions: {},
  content: { description: "人工内容", cover_file_id: "9", confirmed: true } };

describe("batch configuration", () => {
  it('keeps existing Bilibili input and applies defaults only to new allocation rows', () => {
    const saved = { ...candidate, content:{ title:'已保存标题', description:'简介', bilibili:{ tags:['保留'], statement:'内容含营销信息' }, confirmed:true } };
    const rule = { defaults:{ partition_id:1008, partition_name:'游戏', tags:['默认'], visibility:'public' } };
    const rows = batch.reconcileAllocation([saved], [pair('3','4'),pair('5','6')], 'bilibili', rule);
    expect(rows[0]).toBe(saved);
    expect(rows[1].content.bilibili.tags).toEqual(['默认']);
    expect(rows[1].content.bilibili.statement).toBe('内容无需标注');
    rows[1].content.bilibili.tags.push('独立');
    expect(rule.defaults.tags).toEqual(['默认']);
  });
  it('invalidates Bilibili tag suggestions and confirmation when manually editing tags', () => {
    const original={...candidate,content:{title:'标题',bilibili:{tags:['旧标签'],statement:'内容含营销信息'},confirmed:true}};
    const edited=batch.editCandidate(original,{bilibili:{...original.content.bilibili,tags:['新标签']}},'bilibili');
    expect(edited.field_revisions.tags).toBe('1');
    expect(edited.content.confirmed).toBe(false);
    const generated=batch.editCandidate(edited,{tags:'游戏\n画中画'},'bilibili');
    expect(generated.content.bilibili.tags).toEqual(['游戏','画中画']);
    expect(generated.content.bilibili.statement).toBe('内容含营销信息');
    expect(generated.content).not.toHaveProperty('tags');
    expect(generated.field_revisions.tags).toBe('2');
  });
  it('freezes only Bilibili fields in batch input and clears both covers after a video change',()=>{
    const row={...candidate,content:{title:'标题',description:'简介',bilibili:{tags:['游戏']},cover_file_id:'9',space_cover_file_id:'10',portrait_cover_file_id:'0',topics:'',confirmed:true}};
    const form=batch.emptyBatch();form.platform='bilibili';form.config.candidates=[row];
    const input=batch.batchInput(form).config.candidates[0].content;
    expect(input).toHaveProperty('space_cover_file_id','10');
    expect(input).not.toHaveProperty('portrait_cover_file_id');
    expect(input).not.toHaveProperty('topics');
    const next=batch.replaceCandidatePair(row,pair('3','8'));
    expect(next.content).not.toHaveProperty('space_cover_file_id');
    expect(next.content).not.toHaveProperty('cover_file_id');
    expect(next.content.title).toBe('标题');
  });
  it("preserves target count and identity when deleting and reordering candidates", () => {
    const config = batch.emptyBatch();
    config.targets = [{ category_id: "2", target_count: 60 }];
    config.config.candidates = [candidate, { ...candidate, row_key: "second", order: 1 }];
    batch.moveEntry(config.config.candidates, 1, -1);
    config.config.candidates.splice(1, 1);
    expect(batch.batchInput(config).config.candidates).toMatchObject([{ row_key: "second", order: 0 }]);
    expect(batch.candidateShortfalls(config)).toEqual([{ category_id: "2", target_count: 60, candidate_count: 1, shortfall: 59 }]);
  });

  it("keeps reviewed input when allocation returns the same pair", () => {
    expect(batch.reconcileAllocation([candidate], [{ ...pair("3", "4"), row_key: "different" }])).toEqual([candidate]);
  });

  it("invalidates cover references only when replacing the video", () => {
    const nextAccount = batch.replaceCandidatePair(candidate, pair("7", "4"));
    expect(nextAccount).toMatchObject({ row_key: "row-one", input_revision: "9007199254740994", content: { cover_file_id: "9", confirmed: false } });
    const nextVideo = batch.replaceCandidatePair(candidate, pair("3", "8"));
    expect(nextVideo.content).toEqual({ description: "人工内容", confirmed: false });
  });

  it("rejects stale suggestions based on the candidate and field revisions", () => {
    const basis = batch.generationInput(candidate, "baidu", ["description", "topics"], "");
    const edited = batch.editCandidate(candidate, { description: "已修改" });
    expect(batch.matchesCandidateField(edited, basis, "description")).toBe(false);
    expect(batch.matchesCandidateField(edited, basis, "topics")).toBe(true);
    expect(batch.matchesCandidateField(batch.replaceCandidatePair(edited, pair("3", "8")), basis, "topics")).toBe(false);
  });

  it("adopts a suggestion without confirming the content", () => {
    expect(batch.editCandidate(candidate, { topics: "建议话题" }).content).toEqual({
      description: "人工内容", cover_file_id: "9", topics: "建议话题", confirmed: false,
    });
  });

  it("distinguishes unknown availability, failed reads and verified zero", () => {
    expect(batch.displayAvailability({ availability: "empty" })).toBe("暂无可用成片");
    expect(batch.displayAvailability({ availability: "unknown" })).toBe("可用量未知");
    expect(batch.displayAvailability({ error: "读取失败" })).toBe("读取失败");
  });

  it("recovers an ambiguous save using exact query followed by detail", async () => {
    const save = vi.fn().mockRejectedValue(new Error("连接中断"));
    const api = {
      save, list: vi.fn().mockResolvedValue({ items: [{ id: "9007199254740995" }] }),
      get: vi.fn().mockResolvedValue({ id: "9007199254740995", revision: "2" }),
    };
    const original = { request_key: "original", targets: [] };
    await expect(batch.sendNewBatch(api, original)).resolves.toEqual({ id: "9007199254740995", revision: "2" });
    expect(api.list).toHaveBeenCalledWith({ request_key: "original", page: 1, page_size: 1 });
    expect(api.get).toHaveBeenCalledWith("9007199254740995");
    expect(save).toHaveBeenCalledOnce();
  });

  it("queries before retry and resends the same frozen request", async () => {
    const order = [];
    const api = { list: async () => { order.push("list"); return { items: [] }; },
      save: async (body) => { order.push(body); return { id: "6", revision: "1" }; } };
    const original = { request_key: "original", config: { candidates: [] } };
    expect(await batch.sendNewBatch(api, original, { retry: true })).toEqual({ id: "6", revision: "1" });
    expect(order).toEqual(["list", original]);
  });
});

it("does not apply a bulk default to manually edited text without explicit overwrite", () => {
  expect(batch.publicContentPatch(candidate, { description: "公共默认", topics: "共同话题" }))
    .toEqual({ topics: "共同话题" });
  expect(batch.publicContentPatch(candidate, { description: "公共默认" }, true).description).toBe("公共默认");
});

it("recognizes the current resources after an equivalent reorder", () => {
  const config = batch.emptyBatch();
  config.targets = [{ category_id: "2", target_count: 1 }];
  config.config.candidates = [candidate];
  const input = batch.batchInput(config);
  expect(batch.batchFormProblem(config, { requireName: false })).toBe("请填写异常任务上限（正整数）");
  config.execution_setting.exception_task_limit = 1;
  expect(batch.batchFormProblem(config, { requireName: false })).toBe("");
  expect(input.execution_setting.exception_task_limit).toBeNull();
});

it("keeps a row and advances its resource revision when reallocation changes its account", () => {
  const [result] = batch.reconcileAllocation([candidate], [{ ...candidate, ...pair("7", "4"), row_key: "provided" }]);
  expect(result).toMatchObject({ row_key: "row-one", input_revision: "9007199254740994",
    media_account_id: "7", content: { description: "人工内容", cover_file_id: "9", confirmed: false } });
});

it("invalidates outdated template covers while preserving an unrelated image cover", () => {
  const withRender = { ...candidate, cover_title: "旧标题", content: { ...candidate.content,
    cover_render: { title: "旧标题" }, portrait_cover_file_id: "10" } };
  const updated = batch.editCandidate(withRender, { cover_title: "新标题" });
  expect(updated.content).not.toHaveProperty("cover_render");
  expect(updated.content).not.toHaveProperty("cover_file_id");
  expect(updated.content).toMatchObject({ portrait_cover_file_id: "10", confirmed: false });
  expect(updated.field_revisions.cover_render).toBe("1");
  expect(batch.editCandidate(updated, { confirmed: true }).field_revisions.confirmed).toBe("1");
});

it("lets explicit confirmation proceed only for selected rows with no other server issue", () => {
  const rows = [
    { ...candidate, row_key: "ready", content: { confirmed: false } },
    { ...candidate, row_key: "blocked", content: { confirmed: false } },
    { ...candidate, row_key: "already-confirmed" },
  ];
  const issues = [
    { row_key: "ready", code: "confirmation_required", field: "confirmed" },
    { row_key: "blocked", code: "confirmation_required", field: "confirmed" },
    { row_key: "blocked", code: "cover_missing", field: "cover_file_id", message: "请选择所需规格的封面" },
    { row_key: "not-selected", code: "output_occupied", message: "此成片已被占用" },
  ];
  expect(batch.reviewSelectedConfirmations(rows, issues)).toEqual({
    confirmable: ["ready"], blocked: [{ rowKey: "blocked", message: "请选择所需规格的封面" }],
  });
});
