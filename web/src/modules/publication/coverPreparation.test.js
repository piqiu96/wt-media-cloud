import { expect, it } from "vitest";
import {
  coverMatchesRatio,
  coverReference,
  coverRenderInput,
  coverResultMatches,
  selectMatchingCover,
} from "./coverPreparation.js";
const config = {
  method: "template",
  template_id: "9007199254740993",
  published_revision: 2,
  title: "封面标题",
  background_mode: "business",
  background_source: "output",
  background_image_asset_id: "",
};
const output = { id: "71", thumbnail: { file_id: "81" } };
it("pins exactly one business background and preserves string identifiers", () => {
  expect(coverRenderInput(config, output, "row", "4")).toEqual({
    template_id: "9007199254740993",
    expected_published_revision: 2,
    title: "封面标题",
    background_output_id: "71",
    row_key: "row",
    input_revision: "4",
  });
  expect(
    coverRenderInput(
      {
        ...config,
        background_source: "image",
        background_image_asset_id: "12",
      },
      output,
      "row",
      "4",
    ),
  ).toMatchObject({ background_image_asset_id: "12" });
  expect(
    coverRenderInput(
      { ...config, background_mode: "fixed" },
      output,
      "row",
      "4",
    ),
  ).not.toHaveProperty("background_output_id");
  expect(() => coverRenderInput(config, { id: "71" }, "row", "4")).toThrow(
    /成片/,
  );
});
it("only adopts a complete artifact for the same frozen input and request", () => {
  const result = {
    file: { id: "91", resource_path: "/real" },
    render_input_hash: "abc",
    preview: false,
    row_key: "row",
    input_revision: "4",
    snapshot: {
      template_id: config.template_id,
      published_revision: 2,
      title: config.title,
      background_output_id: "71",
      background_image_asset_id: "0",
    },
  };
  const input = coverRenderInput(config, output, "row", "4");
  expect(coverResultMatches(result, input)).toBe(true);
  expect(coverResultMatches(result, { ...input, title: "修改后的标题" })).toBe(
    false,
  );
  expect(coverResultMatches({ ...result, input_revision: "3" }, input)).toBe(
    false,
  );
  expect(coverResultMatches({ ...result, preview: true }, input)).toBe(false);
  expect(coverReference(result)).toEqual({
    template_id: config.template_id,
    published_revision: 2,
    title: config.title,
    background_output_id: "71",
    background_image_asset_id: "0",
    render_input_hash: "abc",
  });
});

it("requires actual matching dimensions and refuses an oversized cover",()=>{
 expect(coverMatchesRatio({width:1280,height:720},16,9)).toBe(true);
 expect(coverMatchesRatio({width:1080,height:1440},3,4)).toBe(true);
 expect(coverMatchesRatio({width:1280,height:720},3,4)).toBe(false);
 expect(coverMatchesRatio({metadata:{width:1280,height:720},size_bytes:5242881},16,9)).toBe(false);
 expect(coverMatchesRatio({},16,9)).toBe(false);
});

it('selects the exact ratio and accepted format across image-library pages',async()=>{
 const calls=[];
 const client={listImageAsset:async input=>{calls.push(input);return {total:101,items:input.page===1?[
  {file_id:'1',resource_path:'/wrong',width:1600,height:900,format:'png',category_ids:['1']},
  {file_id:'2',resource_path:'/webp',width:1200,height:900,format:'webp',category_ids:['1']},
 ]:[{file_id:'3',resource_path:'/correct',width:1200,height:900,format:'png',category_ids:['1']}]}}};
 const selected=await selectMatchingCover(client,'1',{aspect:[4,3]},'bilibili');
 expect(selected.file_id).toBe('3');
 expect(calls.map(c=>c.page)).toEqual([1,2]);
 expect(calls.every(c=>c.aspect==='landscape')).toBe(true);
});
