import { it, expect } from "vitest";
import { documentInput } from "./coverDocument.js";
it("keeps fixed image identity while stripping environment-specific resource URLs", () => {
  const doc = {
    backgroundMode: "fixed",
    imageAssetId: "9007199254740993",
    image: "http://localhost:8188/api/v1/file-resources/token",
    layers: [],
  };
  expect(documentInput(doc)).toEqual({ ...doc, image: "" });
  expect(doc.image).toContain("localhost");
  expect(documentInput({ ...doc, backgroundMode: "business" })).toEqual({
    backgroundMode: "business",
    image: "",
    layers: [],
  });
});
