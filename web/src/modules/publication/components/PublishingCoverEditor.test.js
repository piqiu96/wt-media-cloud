import { createSSRApp, reactive, nextTick } from "vue";
import { renderToString } from "vue/server-renderer";
import { expect, it, vi, beforeEach } from "vitest";
const services = vi.hoisted(() => ({
  getCoverTemplate: vi.fn(),
  listCoverTemplate: vi.fn(),
  renderCover: vi.fn(),
}));
vi.mock("../../../shared/api/image.js", () => ({
  createImageClient: () => services,
}));
import Editor from "./PublishingCoverEditor.vue";
let state, props, emit;
const template = {
  id: "8",
  canvas_width:1280,canvas_height:720,
  enabled: true,
  published_revision: 2,
  published_config: { backgroundMode: "business" },
};
beforeEach(async () => {
  vi.clearAllMocks();
  services.getCoverTemplate.mockResolvedValue(template);
  services.listCoverTemplate.mockResolvedValue({ items: [template] });
  props = reactive({
    aspect:[16,9],
    config: {
      method: "template",
      template_id: "8",
      published_revision: 2,
      title: "实际标题",
      background_mode: "business",
      background_source: "output",
    },
    output: { id: "71", thumbnail: { file_id: "81" } },
    category: "1",
    disabled: false,
  });
  emit = vi.fn();
  await renderToString(
    createSSRApp({
      setup() {
        state = Editor.setup(props, { emit, expose: () => {} });
        return () => null;
      },
    }),
  );
});
it("reserves generation before preflight so a double click cannot start two renders", async () => {
  let finish;
  services.getCoverTemplate.mockImplementation(
    () => new Promise((r) => (finish = r)),
  );
  const pending = state.generate();
  await state.generate();
  expect(services.getCoverTemplate).toHaveBeenCalledTimes(2); // selected-template read + one preflight
  finish(template);
  services.renderCover.mockRejectedValue(new Error("render failed"));
  await pending;
  expect(services.renderCover).toHaveBeenCalledOnce();
  expect(state.error.value).toContain("render failed");
  expect(state.progress.value).toBe("");
  expect(props.config.title).toBe("实际标题");
});
it("discards preflight after the title changes, without starting rendering", async () => {
  let finish;
  services.getCoverTemplate.mockImplementation(
    () => new Promise((r) => (finish = r)),
  );
  const pending = state.generate();
  props.config = { ...props.config, title: "新标题" };
  await nextTick();
  finish(template);
  await pending;
  expect(services.renderCover).not.toHaveBeenCalled();
  expect(emit).not.toHaveBeenCalledWith("rendered", expect.anything());
});
it("does not adopt a completed render after configuration changes", async () => {
  let finish;
  services.renderCover.mockImplementation(
    () => new Promise((r) => (finish = r)),
  );
  const pending = state.generate();
  await nextTick();
  props.config = { ...props.config, title: "新标题" };
  await nextTick();
  finish({ artwork: { file: { id: "91" } } });
  await pending;
  expect(emit).not.toHaveBeenCalledWith("rendered", expect.anything());
});
it("does not restore an old template selection over a newer choice", async () => {
  const b = { ...template, id: "9" };
  state.templates.value = [b];
  let finish;
  services.getCoverTemplate.mockImplementation(
    () => new Promise((r) => (finish = r)),
  );
  const pending = state.selectTemplate("8");
  await state.selectTemplate("9");
  finish(template);
  await pending;
  expect(
    emit.mock.calls
      .filter((c) => c[0] === "update:config")
      .map((c) => c[1].template_id),
  ).toEqual(["9"]);
});

it("finds a matching template beyond a full page of the other ratio", async () => {
  const portrait = { ...template, id: "101", canvas_width: 1080, canvas_height: 1440 };
  const rows = [...Array.from({ length: 100 }, (_, i) => ({ ...template, id: String(i) })), portrait];
  props.aspect = [3, 4];
  services.listCoverTemplate.mockImplementation(async ({ page, page_size }) => ({
    items: rows.slice((page - 1) * page_size, page * page_size), total: rows.length,
  }));
  await state.loadTemplate();
  expect(state.templates.value).toEqual([portrait]);
});

it("loads further matching templates without excluding other resolutions of the same ratio", async () => {
  const rows = Array.from({ length: 101 }, (_, i) => ({ ...template, id: String(i) }));
  services.listCoverTemplate.mockImplementation(async ({ page, page_size }) => ({
    items: rows.slice((page - 1) * page_size, page * page_size), total: rows.length,
  }));
  await state.loadTemplate("标题");
  expect(state.templates.value).toHaveLength(100);
  expect(state.hasMoreTemplates.value).toBe(true);
  await state.loadMoreTemplates();
  expect(state.templates.value).toEqual(rows);
  expect(state.hasMoreTemplates.value).toBe(false);
  expect(services.listCoverTemplate).toHaveBeenLastCalledWith(expect.objectContaining({ search: "标题", page: 2 }));
});

it("resets pagination for a new search and ignores an older in-flight page", async () => {
  services.listCoverTemplate.mockResolvedValue({ items: [template], total: 101 });
  await state.loadTemplate("旧搜索");
  let finish;
  services.listCoverTemplate.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  const pending = state.loadMoreTemplates();
  const newer = { ...template, id: "new" };
  services.listCoverTemplate.mockResolvedValue({ items: [newer], total: 1 });
  await state.loadTemplate("新搜索");
  finish({ items: [{ ...template, id: "old" }], total: 101 });
  await pending;
  expect(state.templates.value).toEqual([newer]);
  expect(state.hasMoreTemplates.value).toBe(false);
  expect(services.listCoverTemplate).toHaveBeenLastCalledWith(expect.objectContaining({ search: "新搜索", page: 1 }));
});

it("retries the failed next page without losing loaded templates", async () => {
  services.listCoverTemplate.mockResolvedValue({ items: [template], total: 101 });
  await state.loadTemplate("保留搜索");
  services.listCoverTemplate.mockRejectedValueOnce(new Error("读取失败"));
  await state.loadMoreTemplates();
  expect(state.templates.value).toEqual([template]);
  expect(state.error.value).toBe("读取失败");
  const next = { ...template, id: "next" };
  services.listCoverTemplate.mockResolvedValue({ items: [next], total: 101 });
  await state.loadMoreTemplates();
  expect(state.templates.value).toEqual([template, next]);
  expect(services.listCoverTemplate).toHaveBeenLastCalledWith(expect.objectContaining({ search: "保留搜索", page: 2 }));
});

it("stops paging when all templates have a different ratio", async () => {
  services.listCoverTemplate.mockResolvedValue({
    items: [{ ...template, canvas_width: 1080, canvas_height: 1440 }], total: 1,
  });
  await state.loadTemplate();
  expect(state.templates.value).toEqual([]);
  expect(state.hasMoreTemplates.value).toBe(false);
});

it("skips intervening pages of other ratios when loading more", async () => {
  const portrait = { ...template, canvas_width: 1080, canvas_height: 1440 };
  services.listCoverTemplate.mockImplementation(async ({ page }) => ({
    items: page === 2 ? Array(100).fill(portrait) : [{ ...template, id: String(page) }],
    total: 201,
  }));
  await state.loadTemplate();
  await state.loadMoreTemplates();
  expect(state.templates.value.map((item) => item.id)).toEqual(["1", "3"]);
  expect(state.hasMoreTemplates.value).toBe(false);
});

it("prevents duplicate next-page requests while loading", async () => {
  services.listCoverTemplate.mockResolvedValue({ items: [template], total: 101 });
  await state.loadTemplate();
  let finish;
  services.listCoverTemplate.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  const pending = state.loadMoreTemplates();
  const count = services.listCoverTemplate.mock.calls.length;
  await state.loadMoreTemplates();
  expect(services.listCoverTemplate).toHaveBeenCalledTimes(count);
  finish({ items: [{ ...template, id: "next" }], total: 101 });
  await pending;
  expect(state.templates.value).toHaveLength(2);
});
