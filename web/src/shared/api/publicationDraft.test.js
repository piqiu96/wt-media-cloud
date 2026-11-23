import { beforeEach, expect, it, vi } from "vitest";
import {
  createPublicationDraftClient,
  resetPublicationDraftSession,
} from "./publicationDraft.js";

beforeEach(async () => {
  await resetPublicationDraftSession({ tauri: false });
});
it("keeps Web input in application memory, isolated by user, without native or browser persistence", async () => {
  const invokeImpl = vi.fn();
  const first = createPublicationDraftClient({
    userId: "9",
    tauri: false,
    invokeImpl,
  });
  await first.get();
  first.remember({ description: "人工稿" });
  expect(
    (await createPublicationDraftClient({ userId: "9", tauri: false }).get())
      .payload.description,
  ).toBe("人工稿");
  expect(
    (await createPublicationDraftClient({ userId: "10", tauri: false }).get())
      .payload,
  ).toBeNull();
  await resetPublicationDraftSession({ tauri: false });
  expect(
    (await createPublicationDraftClient({ userId: "9", tauri: false }).get())
      .payload,
  ).toBeNull();
  expect(invokeImpl).not.toHaveBeenCalled();
});
it("explicitly saves Desktop input using its revision and clears only after confirmed success", async () => {
  const invokeImpl = vi
    .fn()
    .mockResolvedValueOnce({
      payload: null,
      revision: "4",
      saved_at: null,
      schema_version: 1,
    })
    .mockResolvedValueOnce({
      payload: { description: "人工稿" },
      revision: "5",
      saved_at: "now",
      schema_version: 1,
    })
    .mockResolvedValueOnce({
      payload: null,
      revision: "6",
      saved_at: "later",
      schema_version: 1,
    });
  const client = createPublicationDraftClient({
    userId: "9",
    tauri: true,
    invokeImpl,
  });
  await client.get();
  client.remember({ description: "人工稿" });
  expect(invokeImpl).toHaveBeenCalledTimes(1);
  await client.save({ description: "人工稿" });
  expect(invokeImpl).toHaveBeenNthCalledWith(
    2,
    "local_publication_draft_save",
    {
      args: {
        user_id: "9",
        expected_revision: "4",
        payload: { description: "人工稿" },
      },
    },
  );
  await client.clear();
  expect(invokeImpl).toHaveBeenNthCalledWith(
    3,
    "local_publication_draft_clear",
    { args: { user_id: "9", expected_revision: "5" } },
  );
});
it("preserves current input after a conflict and never resends with a guessed revision", async () => {
  const invokeImpl = vi
    .fn()
    .mockResolvedValueOnce({ payload: null, revision: "1", schema_version: 1 })
    .mockRejectedValueOnce({ code: "draft_revision_conflict" });
  const client = createPublicationDraftClient({
    userId: "9",
    tauri: true,
    invokeImpl,
  });
  await client.get();
  await expect(client.save({ description: "保留我" })).rejects.toMatchObject({
    code: "draft_revision_conflict",
  });
  expect((await client.get()).payload.description).toBe("保留我");
  expect(invokeImpl).toHaveBeenCalledTimes(2);
});
it("rejects a previous session response after logout", async () => {
  let finish;
  const invokeImpl = vi.fn(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const client = createPublicationDraftClient({
    userId: "9",
    tauri: true,
    invokeImpl,
  });
  const pending = client.get();
  await resetPublicationDraftSession({ tauri: false });
  finish({
    payload: { description: "旧账号" },
    revision: "1",
    schema_version: 1,
  });
  await expect(pending).rejects.toMatchObject({
    code: "draft_identity_required",
  });
});
