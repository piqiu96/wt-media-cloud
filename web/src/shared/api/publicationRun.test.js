import { expect, it, vi } from "vitest";
import { handlePublicationHuman } from "./publicationRun.js";
const task = {
  id: "9007199254740993",
  current_attempt_id: "9007199254740995",
  revision: "7",
};
it("uses only the controlled native command and keeps string identities", async () => {
  const invoke = vi
    .fn()
    .mockResolvedValue({
      attempt_id: task.current_attempt_id,
      status: "running",
    });
  await handlePublicationHuman({
    task,
    userId: "9",
    resume: true,
    tauri: true,
    invokeImpl: invoke,
  });
  expect(invoke).toHaveBeenCalledWith("local_publication_resume", {
    args: {
      user_id: "9",
      publication_id: task.id,
      attempt_id: task.current_attempt_id,
      expected_revision: "7",
    },
  });
});
it("requires Desktop and canonical identities before any invocation", async () => {
  const invoke = vi.fn();
  await expect(
    handlePublicationHuman({
      task,
      userId: "9",
      tauri: false,
      invokeImpl: invoke,
    }),
  ).rejects.toThrow("安装版");
  await expect(
    handlePublicationHuman({
      task: { ...task, current_attempt_id: 3 },
      userId: "9",
      tauri: true,
      invokeImpl: invoke,
    }),
  ).rejects.toThrow("任务");
  expect(invoke).not.toHaveBeenCalled();
});
