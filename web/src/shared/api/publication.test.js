import { it, expect } from "vitest";
import { createPublicationClient } from "./publication.js";
it("preserves string IDs, request key and expected revision across create/read/edit", async () => {
  const calls = [];
  const client = createPublicationClient({
    fetch: async (url, init = {}) => {
      calls.push({ url, init });
      return {
        json: async () => ({
          errcode: 0,
          data: { id: "9007199254740993", revision: "1" },
        }),
      };
    },
  });
  const input = {
    request_key: "stable-key",
    platform: "baidu",
    category_id: "4",
    media_account_id: "9",
    composite_output_id: "128",
    content: {
      description: "文案",
      cover_file_id: "5",
      declaration: "无需声明",
      confirmed: true,
    },
  };
  await client.create(input);
  await client.get("9007199254740993");
  await client.edit("9007199254740993", {
    expected_revision: "1",
    content: input.content,
  });
  expect(JSON.parse(calls[0].init.body)).toEqual(input);
  expect(calls[1].url).toContain("/publication/9007199254740993");
  expect(JSON.parse(calls[2].init.body).expected_revision).toBe("1");
});

it("authorizes separately with the reviewed revision and reads original attempts", async()=>{
 const calls=[];const client=createPublicationClient({fetch:async(url,init={})=>{calls.push({url,init});return {json:async()=>({errcode:0,data:[]})}}});
 await client.start("9007199254740993",{expected_revision:"9007199254740994"});await client.attempt("9007199254740993");
 expect(calls[0].url).toContain("/publication/9007199254740993/start");expect(JSON.parse(calls[0].init.body)).toEqual({expected_revision:"9007199254740994"});
 expect(calls[1].url).toContain("/publication/9007199254740993/attempt");expect(calls[1].init.method).toBe("GET");
});

it('keeps resolution evidence, current attempt and safe-retry revision exact', async()=>{
 const calls=[];const client=createPublicationClient({fetch:async(url,init={})=>{calls.push({url,init});return {json:async()=>({errcode:0,data:{}})}}});
 const input={attempt_id:'9007199254740993',expected_revision:'7',conclusion:'failed',evidence:{platform_uid:'u',reason:'失败',basis:'原账号与执行现场已经实际核实',scene_closed:true,no_publishable_work:true}};
 await client.resolve('9007199254740994',input);await client.retry('9007199254740994',{expected_revision:'8'});
 expect(calls[0].url).toContain('/publication/9007199254740994/resolve');expect(JSON.parse(calls[0].init.body)).toEqual(input);
 expect(calls[1].url).toContain('/publication/9007199254740994/retry');expect(JSON.parse(calls[1].init.body)).toEqual({expected_revision:'8'});
});
