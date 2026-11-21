import { describe, it, expect, vi } from "vitest";
import { createImageUpload } from "./uploadImage.js";
describe("image upload recovery", () => {
  it("keeps uploaded file and form attributes when registration fails", async () => {
    const files = {
      upload: vi.fn().mockResolvedValue({ id: "9007199254740993" }),
    };
    const image = {
      registerImageAsset: vi
        .fn()
        .mockRejectedValueOnce(new Error("offline"))
        .mockResolvedValue({ id: "2" }),
    };
    const upload = createImageUpload(files, image),
      file = new File(["image"], "picture.png");
    upload.reset(file);
    await expect(upload.save({ name: "原名" })).rejects.toThrow("offline");
    await expect(upload.save({ name: "修改后的名称" })).resolves.toEqual({
      id: "2",
    });
    expect(files.upload).toHaveBeenCalledTimes(1);
    expect(image.registerImageAsset.mock.calls[1][0]).toEqual({
      name: "修改后的名称",
      file_id: "9007199254740993",
    });
    upload.reset(new File(["other"], "new.png"));
    await upload.save({ name: "新的图片" });
    expect(files.upload).toHaveBeenCalledTimes(2);
  });
});
