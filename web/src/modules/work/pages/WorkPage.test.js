import { createRenderer, nextTick, ssrContextKey } from "vue";
import { beforeEach, afterEach, it, expect, vi } from "vitest";
const mocks = vi.hoisted(() => ({
  work: { list: vi.fn(), get: vi.fn(), summary: vi.fn() },
  router: { push: vi.fn(), replace: vi.fn() },
  query: {},
}));
vi.mock("../../../shared/api/work.js", () => ({
  createWorkClient: () => mocks.work,
}));
vi.mock("vue-router", () => ({
  useRoute: () => ({ query: mocks.query }),
  useRouter: () => mocks.router,
}));
vi.mock("../../../shared/api/compose.js", () => ({
  createComposeClient: () => ({ listCategories: async () => [] }),
}));
vi.mock("../../../shared/api/mediaAccounts.js", () => ({
  createMediaAccountClient: () => ({ listPage: async () => ({ items: [] }) }),
}));
import WorkPage from "./WorkPage.vue";
let state, app;
const renderer = createRenderer({
  createComment: () => ({}),
  insert: () => {},
  remove: () => {},
  parentNode: () => null,
  nextSibling: () => null,
});
beforeEach(async () => {
  vi.clearAllMocks();
  mocks.work.summary.mockResolvedValue({ all: 1, today: 1 });
  mocks.work.list.mockResolvedValue({ items: [], total: 0 });
  app = renderer.createApp({
    setup(p, c) {
      state = WorkPage.setup(p, c);
      return () => null;
    },
  });
  app.provide(ssrContextKey, {});
  app.mount({});
  await nextTick();
  await nextTick();
});
afterEach(() => app.unmount());
it("keeps the latest detail when an older request arrives late", async () => {
  let finish;
  mocks.work.get
    .mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    )
    .mockResolvedValueOnce({ id: "2", title: "当前作品" });
  const old = state.openDetail({ id: "1" });
  await state.openDetail({ id: "2" });
  finish({ id: "1", title: "旧作品" });
  await old;
  expect(state.detail.value.id).toBe("2");
});
it("returning to list prevents a delayed detail from opening itself", async () => {
  let finish;
  mocks.work.get.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const pending = state.openDetail({ id: "1" });
  state.back();
  finish({ id: "1" });
  await pending;
  expect(state.detail.value).toBe(null);
});
it("queries actual publication dates and displays scoped today count", async () => {
  state.filters.value = {
    search: "三角洲",
    platform: "baidu",
    category_id: "1",
    media_account_id: "9",
    published_range: ["2026-11-20", "2026-11-20"],
  };
  state.query();
  await new Promise((resolve) => setTimeout(resolve, 0));
  expect(mocks.work.list).toHaveBeenCalledWith(
    expect.objectContaining({
      published_from: "2026-11-19T16:00:00.000Z",
      published_before: "2026-11-20T16:00:00.000Z",
      media_account_id: "9",
      search: "三角洲",
    }),
  );
  expect(mocks.work.summary).toHaveBeenCalledWith(
    mocks.work.list.mock.calls[0][0],
  );
  expect(state.summary.value.today).toBe(1);
});
it("opens the exact source task without changing the work record", () => {
  state.sourceTask({ publication_id: "9007199254740993" });
  expect(mocks.router.push).toHaveBeenCalledWith({
    path: "/publish",
    query: { publication_id: "9007199254740993" },
  });
  expect(mocks.work.get).not.toHaveBeenCalled();
});

it("retries the exact failed detail and clears a failed deep link on return", async () => {
  mocks.work.get
    .mockRejectedValueOnce(new Error("暂时无法读取指定作品"))
    .mockResolvedValueOnce({ id: "123" });
  await state.openDetail({ id: "123" });
  expect(state.detail.value).toBe(null);
  await state.retryDetail();
  expect(mocks.work.get.mock.calls).toEqual([["123"], ["123"]]);
  expect(state.detail.value.id).toBe("123");
  mocks.query.work_id = "123";
  state.back();
  expect(mocks.router.replace).toHaveBeenCalledWith({
    path: "/work",
    query: {},
  });
  delete mocks.query.work_id;
});
it("restores the list scroll position after returning from a shorter detail", async () => {
  const host = { scrollTop: 720 };
  state.pageRoot.value = { closest: () => host };
  mocks.work.get.mockResolvedValue({ id: "1" });
  await state.openDetail({ id: "1" });
  await nextTick();
  await nextTick();
  expect(host.scrollTop).toBe(0);
  state.back();
  await nextTick();
  await nextTick();
  expect(host.scrollTop).toBe(720);
});
