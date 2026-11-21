import { expect, it } from "vitest";
import {
  coverReference,
  coverRenderInput,
  coverResultMatches,
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
