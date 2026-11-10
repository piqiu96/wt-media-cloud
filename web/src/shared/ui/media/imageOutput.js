// Browser-side image operations; callers own platform specifications and storage.
export async function fitImage(src, width, height) {
  if (
    !src ||
    !Number.isFinite(width) ||
    !Number.isFinite(height) ||
    width < 1 ||
    height < 1
  )
    throw new Error("图片或目标尺寸无效");
  const image = await new Promise((resolve, reject) => {
    const im = new Image();
    im.onload = () => resolve(im);
    im.onerror = () => reject(new Error("图片读取失败，请更换图片"));
    im.src = src;
  });
  const scale = Math.min(
    width / image.naturalWidth,
    height / image.naturalHeight,
  );
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext("2d");
  context.fillStyle = "#172f4c";
  context.fillRect(0, 0, width, height);
  context.drawImage(
    image,
    (width - image.naturalWidth * scale) / 2,
    (height - image.naturalHeight * scale) / 2,
    image.naturalWidth * scale,
    image.naturalHeight * scale,
  );
  return canvas.toDataURL("image/png");
}
