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
 await client.start("9007199254740993",{expected_revision:"9007199254740994"});await client.attempts("9007199254740993");
 expect(calls[0].url).toContain("/publication/9007199254740993/start");expect(JSON.parse(calls[0].init.body)).toEqual({expected_revision:"9007199254740994"});
 expect(calls[1].url).toContain("/publication/9007199254740993/attempts");expect(calls[1].init.method).toBe("GET");
});
