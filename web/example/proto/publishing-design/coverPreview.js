import { coverSignature, coverSpecs } from "./covers.js";

export function coverPreviewState(row) {
  const templated = ["模板合成", "规则生成封面", "图片库模板"].includes(
    row?.coverMode,
  );
  const pending =
    templated &&
    (row.coverSignature !== coverSignature(row) || !!row.coverError);
  const specs = coverSpecs(row?.platform);
  const images = specs.length
    ? specs.map((spec) => ({
        ...spec,
        src:
          pending || row?.coverError
            ? ""
            : row?.coverVariants?.[spec.key] ||
              (templated ? "" : row?.coverSource || row?.cover),
      }))
    : [
        {
          key: "source",
          label: "当前封面",
          src: pending || row?.coverError ? "" : row?.cover,
        },
      ];
  return { pending, images };
}
