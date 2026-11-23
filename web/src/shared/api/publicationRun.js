import { invoke } from "@tauri-apps/api/core";
export const nativePublicationDesktop = () =>
  typeof window !== "undefined" && !!window.__TAURI_INTERNALS__;
export async function handlePublicationHuman({
  task,
  userId,
  resume = false,
  tauri = nativePublicationDesktop(),
  invokeImpl = invoke,
}) {
  if (!tauri) throw new Error("请在原任务所属电脑的安装版中处理人工验证");
  const values = [userId, task?.id, task?.current_attempt_id, task?.revision];
  if (
    !values.every(
      (value) =>
        typeof value === "string" &&
        /^[1-9][0-9]*$/.test(value) &&
        BigInt(value) <= 18446744073709551615n,
    )
  )
    throw new Error("任务或登录身份已变化，请刷新后重试");
  return invokeImpl(
    resume ? "local_publication_resume" : "local_publication_scene",
    {
      args: {
        user_id: userId,
        publication_id: task.id,
        attempt_id: task.current_attempt_id,
        expected_revision: task.revision,
      },
    },
  );
}
