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
