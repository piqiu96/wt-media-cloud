import { describe, it, expect } from "vitest";
import {
  definitivePublicationRejection,
  accountUnavailable,
  outputUnavailable,
  contentProblem,
} from "./eligibility.js";
describe("publication preparation eligibility", () => {
  const account = {
    platform: "baidu",
    enabled: true,
    identification_status: "identified",
    platform_account_id: "uid",
    browser_profile_id: "profile",
    platform_login_status: "normal",
    category_ids: [4],
    team_id: 1,
  };
  it("explains incompatible category and account identity without assigning resources", () => {
    expect(accountUnavailable(account, 4)).toBe("");
    expect(accountUnavailable(account, 1)).toContain("类目");
    expect(
      accountUnavailable({ ...account, platform_account_id: "" }, 4),
    ).toContain("真实账号");
    expect(
      outputUnavailable(
        { category_id: 1, team_id: 1, video: { status: "ready" } },
        4,
        account,
      ),
    ).toContain("类目");
    expect(
      outputUnavailable(
        { category_id: 4, team_id: 2, video: { status: "ready" } },
        4,
        account,
      ),
    ).toContain("团队");
  });
  it("requires an explicit content confirmation", () => {
    const form = {
      category_id: "4",
      media_account_id: "9",
      composite_output_id: "128",
      content: {
        description: "实测文案",
        cover_file_id: "5",
        declaration: "无需声明",
        confirmed: false,
      },
    };
    expect(contentProblem(form)).toContain("确认");
    expect(
      contentProblem({
        ...form,
        content: { ...form.content, confirmed: true },
      }),
    ).toBe("");
  });
});

describe("uncertain publication save",()=>{
 it("retains original input after a transport, parse or server failure",()=>{
  for(const error of [new Error("offline"),{errcode:10001},{errcode:18006}])expect(definitivePublicationRejection(error)).toBe(false)
  for(const errcode of [18001,18002,18003,18004,18005])expect(definitivePublicationRejection({errcode})).toBe(true)
 })
})

it('does not offer a server-occupied output even while its production status is unpublished', () => {
 const output={category_id:4,team_id:1,platforms:['baidu'],video:{status:'ready'},publish_status:'pending_publish'};
 expect(outputUnavailable({...output,occupied:true},4,{team_id:1})).toBe('该成片已被当前平台的任务占用');
 expect(outputUnavailable({...output,occupied:false},4,{team_id:1})).toBe('');
});
