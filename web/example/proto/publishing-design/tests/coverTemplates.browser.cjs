var __create = Object.create;
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __getProtoOf = Object.getPrototypeOf;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toESM = (mod, isNodeMode, target) => (target = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(
  // If the importer is in node compatibility mode or this is not an ESM
  // file that has been converted to a CommonJS file using a Babel-
  // compatible transform (i.e. "__esModule" has not been set), then set
  // "default" to the CommonJS "module.exports" for node compatibility.
  isNodeMode || !mod || !mod.__esModule ? __defProp(target, "default", { value: mod, enumerable: true }) : target,
  mod
));
var __create2 = Object.create;
var __defProp2 = Object.defineProperty;
var __getOwnPropDesc2 = Object.getOwnPropertyDescriptor;
var __getOwnPropNames2 = Object.getOwnPropertyNames;
var __getProtoOf2 = Object.getPrototypeOf;
var __hasOwnProp2 = Object.prototype.hasOwnProperty;
var __copyProps2 = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames2(from))
      if (!__hasOwnProp2.call(to, key) && key !== except)
        __defProp2(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc2(from, key)) || desc.enumerable });
  }
  return to;
};
var __toESM2 = (mod, isNodeMode, target) => (target = mod != null ? __create2(__getProtoOf2(mod)) : {}, __copyProps2(
  // If the importer is in node compatibility mode or this is not an ESM
  // file that has been converted to a CommonJS file using a Babel-
  // compatible transform (i.e. "__esModule" has not been set), then set
  // "default" to the CommonJS "module.exports" for node compatibility.
  isNodeMode || !mod || !mod.__esModule ? __defProp2(target, "default", { value: mod, enumerable: true }) : target,
  mod
));
const { chromium } = require("playwright");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path"), { pathToFileURL } = require("node:url");
const evidence = process.env.SCREENSHOT_DIR || path.join(require("node:os").tmpdir(), "wt-cover-v1-evidence");
fs.mkdirSync(evidence, { recursive: true });
(async () => {
  const browser = await chromium.launch({ headless: true, ...process.env.CHROMIUM_EXECUTABLE ? { executablePath: process.env.CHROMIUM_EXECUTABLE } : {} });
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();
  page.setDefaultTimeout(5e3);
  const errors = [], passed = [];
  page.on("pageerror", (e) => {
    errors.push(e.message);
    console.log("PAGEERROR", e.stack);
  });
  let snapshot = { revision: 1, data: (await import(pathToFileURL(path.resolve(__dirname, "../model.js")).href)).seed() }, fail = false, responseDelay = 0;
  await page.route("**/__example/publishing/**", async (r) => {
    if (responseDelay && r.request().method() !== "GET") await new Promise((resolve) => setTimeout(resolve, responseDelay));
    if (fail && r.request().method() !== "GET") return r.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ message: "走查注入的保存失败" }) });
    if (r.request().method() !== "GET") snapshot = { revision: snapshot.revision + 1, data: r.request().postDataJSON().data };
    return r.fulfill({ contentType: "application/json", body: JSON.stringify(snapshot) });
  });
  const shot = async (name) => {
    await page.evaluate(() => document.querySelectorAll(".t-message__close").forEach((el) => el.click()));
    await page.waitForTimeout(350);
    await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
    await page.screenshot({ path: evidence + "/" + name + ".png" });
  }, btn = (name) => page.getByRole("button", { name, exact: true }), layer = () => page.locator(".editor-layers .layer-name").filter({ hasText: "动态主标题" }), footer = () => page.locator(".cover-template-page footer"), listRow = () => page.getByRole("row").filter({ has: page.getByText("V1 走查模板", { exact: true }) }).first(), saved = () => page.locator(".cover-save-status").filter({hasText:/^已保存$/}).waitFor(), record = () => snapshot.data.templates.find((t) => t.name === "V1 走查模板");
  function pass(name) {
    passed.push(name);
    console.log("PASS", name);
  }
  async function save() {
    await footer().getByRole("button", { name: "保存草稿", exact: true }).click();
    await saved();
    await page.waitForTimeout(250);
  }
  async function leaveEditor() {
    await btn("返回").click();
    await page.getByRole("heading", { name: "封面模板", exact: true }).waitFor();
    await page.waitForTimeout(250);
  }
  try {
    await page.goto((process.env.EXAMPLE_URL || "http://127.0.0.1:8768") + "/example/?view=publishing#templates");
    const broken = await page.evaluate(async () => {
      const m = await import("/example/proto/publishing-design/coverTemplates.js");
      const d = m.contentOf(m.createTemplate({ id: 5, name: "broken" }));
      d.backgroundMode = "fixed";
      d.image = "data:image/png;base64,broken";
      const r = await m.checkDocument(d);
      return { image: !!r.image, issues: r.issues };
    });
    console.log("BROKEN", JSON.stringify(broken));
    assert(broken.issues.some((i) => i.level === "error"), "失效图片必须阻止发布");
    await btn("新建模板").click();
    await page.getByPlaceholder("请输入模板名称").fill("V1 走查模板");
    await shot("01-create");
    await btn("创建并编辑").click();
    await page.locator(".cover-editor-heading").waitFor();
    assert.equal(record().enabled, false);
    assert.equal(record().version, 0);
    await shot("02-editor-1440");
    pass("新建基础信息并进入独立页面");
    await layer().click();
    assert.equal(await btn("动态主标题").count(),0);
    await page.locator(".editor-properties-panel textarea").fill("保存期间继续编辑");
    await save();
    pass("单动态标题限制及草稿保存");
    responseDelay = 650;
    await page.locator(".editor-properties-panel textarea").fill("发送时的草稿");
    await footer().getByRole("button", { name: "保存草稿", exact: true }).click();
    await page.locator(".editor-properties-panel textarea").fill("保存期间继续编辑");
    await page.waitForTimeout(950);
    responseDelay = 0;
    assert.equal(await page.locator(".cover-save-status").filter({hasText:/^未保存$/}).count(), 1, "保存期间的后续编辑不能被标为已保存");
    await save();
    pass("延迟保存期间的编辑仍保留未保存标记");
    await footer().getByRole("button", { name: "预览与校验", exact: true }).click();
    await page.locator(".cover-preview-stage img").waitFor();
    const short = await page.locator(".cover-preview-stage img").getAttribute("src");
    await shot("03a-preview-short");
    const downloadReady = page.waitForEvent("download");
    await btn("导出预览 PNG").click();
    const download = await downloadReady;
    assert(download.suggestedFilename().startsWith("预览样例-"));
    pass("预览 PNG 导出带样例标识");
    await page.getByText("较长", { exact: true }).click();
    await page.waitForTimeout(400);
    assert.notEqual(await page.locator(".cover-preview-stage img").getAttribute("src"), short);
    await page.getByText("自定义", { exact: true }).click();
    await page.getByPlaceholder("输入预览标题").fill("超长标题".repeat(180));
    await page.locator(".cover-issue.error").waitFor();
    await shot("03-overflow");
    await page.locator(".cover-issue.error").getByRole("button", { name: "定位修改" }).click();
    await page.locator(".editor-properties-panel textarea").waitFor();
    assert.equal(await page.locator(".editor-properties-panel textarea").inputValue(), "保存期间继续编辑");
    pass("短长自定义预览及溢出定位，样例与设计隔离");
    const x = page.locator(".editor-properties-panel .editor-field").filter({ hasText: "X 坐标" }).locator("input");
    await x.fill("0");
    await x.press("Tab");
    await save();
    assert.equal(record().draft.preview, "", "修改设计后旧预览必须失效");
    await leaveEditor();
    assert((await listRow().locator(".t-switch").getAttribute("class")).includes("t-is-disabled"));
    await listRow().getByRole("button", { name: "发布", exact: true }).click();
    await page.getByText("文字超出画布或安全区", { exact: true }).waitFor();
    assert(await btn("确认发布").isDisabled());
    await page.locator(".cover-publish-issue").getByRole("button", { name: "定位修改" }).first().click();
    await page.locator(".editor-properties-panel textarea").waitFor();
    await x.fill("86");
    await x.press("Tab");
    await save();
    await leaveEditor();
    pass("列表发布阻断及定位修复");
    await listRow().getByRole("button", { name: "发布", exact: true }).click();
    await page.getByText("校验通过，可以发布。", { exact: true }).waitFor();
    await shot("04-publish");
    await btn("确认发布").click();
    await page.waitForTimeout(400);
    assert.equal(record().version, 1);
    assert.equal(record().enabled, false);
    assert.equal(record().draft, null);
    pass("首次发布保持停用");
    await listRow().locator(".t-switch").click();
    await btn("启用").click();
    await page.waitForTimeout(400);
    assert.equal(record().enabled, true);
    const original = JSON.stringify(record().document);
    await listRow().getByRole("button", { name: "编辑", exact: true }).click();
    if (!record().draft) await page.locator(".cover-save-status").filter({hasText:/^已保存$/}).waitFor();
    await layer().click();
    await page.locator(".editor-properties-panel textarea").fill("发布更新样例");
    await save();
    assert.equal(JSON.stringify(record().document), original);
    assert.equal(record().enabled, true);
    await leaveEditor();
    await listRow().getByText("有草稿", { exact: true }).waitFor();
    pass("编辑保存隔离已发布配置，使用状态不变");
    await listRow().getByRole("button", { name: "发布更新", exact: true }).click();
    await page.getByText("校验通过，可以发布。", { exact: true }).waitFor();
    fail = true;
    await btn("确认发布").click();
    await page.getByText(/原已发布内容及启停状态保持不变/).waitFor();
    assert.equal(record().version, 1);
    assert.equal(record().enabled, true);
    fail = false;
    await btn("确认发布").click();
    await page.waitForTimeout(500);
    assert.equal(record().version, 2);
    assert.equal(record().enabled, true);
    pass("发布失败保留原配置，重试成功保持启用");
    await listRow().locator(".t-switch").click();
    await btn("取消").last().click();
    assert.equal(record().enabled, true);
    await listRow().locator(".t-switch").click();
    fail = true;
    await btn("停用").click();
    await page.waitForTimeout(400);
    assert.equal(record().enabled, true);
    fail = false;
    await listRow().locator(".t-switch").click();
    await btn("停用").click();
    await page.waitForTimeout(400);
    assert.equal(record().enabled, false);
    await listRow().locator(".t-switch").click();
    await btn("启用").click();
    await page.waitForTimeout(400);
    assert.equal(record().enabled, true);
    pass("Switch 取消、失败回退、停用及重新启用");
    await listRow().getByRole("button", { name: "编辑", exact: true }).click();
    await layer().click();
    await page.locator(".editor-properties-panel textarea").fill("保留失败时的修改");
    fail = true;
    await footer().getByRole("button", { name: "保存草稿", exact: true }).click();
    await page.locator(".cover-save-status").filter({hasText:/^保存失败$/}).waitFor();
    await page.waitForTimeout(150);
    const failedGrid=await page.locator('.editor-grid').boundingBox(),failedFooter=await footer().boundingBox();
    assert(failedGrid.y+failedGrid.height<failedFooter.y,'保存失败提示不能把画布推入 Footer');
    assert.equal(await page.locator(".editor-properties-panel textarea").inputValue(), "保留失败时的修改");
    assert.equal(record().draft, null);
    await btn("返回").click();
    await btn("继续编辑").click();
    assert.equal(await page.locator(".editor-properties-panel textarea").inputValue(), "保留失败时的修改");
    fail = false;
    await btn("返回").click();
    await btn("保存并离开").click();
    await page.getByRole("heading", { name: "封面模板", exact: true }).waitFor();
    assert(record().draft);
    pass("保存失败保留修改及三选离开确认");
    await listRow().getByRole("button", { name: "编辑", exact: true }).click();
    await layer().click();
    assert.equal(await page.locator(".editor-properties-panel textarea").inputValue(), "保留失败时的修改");
    await page.locator(".editor-properties-panel textarea").fill("应该放弃的修改");
    await btn("返回").click();
    await btn("放弃修改").click();
    await listRow().getByRole("button", { name: "复制", exact: true }).click();
    const dialog = page.locator(".resource-overlay-dialog:visible");
    await dialog.locator(".t-select__wrap").first().click();
    await page.getByText("16:9 · 1920 × 1080", { exact: true }).last().click();
    await btn("复制并编辑").click();
    await page.locator(".cover-editor-heading").waitFor();
    const clone = snapshot.data.templates.find((t) => t.name === "V1 走查模板 副本");
    assert.equal(clone.version, 0);
    assert.equal(clone.enabled, false);
    assert.equal(clone.draft.document.width, 1920);
    assert.equal(clone.draft.document.layers[0].fontSize, record().draft.document.layers[0].fontSize);
    pass("放弃修改及复制新规格");
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.waitForTimeout(500);
    await shot("05-editor-1280");
    const bounds = await page.locator(".cover-template-page").evaluate((el) => ({ footer: el.querySelector("footer").getBoundingClientRect().top, canvas: el.querySelector(".editor-stage").getBoundingClientRect().bottom, overflow: document.documentElement.scrollWidth > innerWidth, footerBottom: el.querySelector("footer").getBoundingClientRect().bottom, viewport: innerHeight }));
    console.log("BOUNDS", bounds);
    assert.equal(bounds.overflow, false);
    assert(bounds.footerBottom <= bounds.viewport);
    assert(bounds.canvas <= bounds.footer);
    await btn("折叠图层").click();
    await shot("06-collapsed-1280");
    pass("1280 布局、Footer 无遮挡及折叠图层");
    await leaveEditor();
    await page.setViewportSize({ width: 1440, height: 900 });
    await shot("07-list-final");
    await listRow().getByRole("button", { name: "编辑", exact: true }).click();
    await page.reload();
    await page.locator(".cover-editor-heading").waitFor();
    await layer().click();
    assert.equal(await page.locator(".editor-properties-panel textarea").inputValue(), "保留失败时的修改");
    pass("重新进入及刷新编辑页面");
    await btn("固定文字").click();
    await page.locator(".editor-properties-panel textarea").fill("固定标语");
    const fixedRow = page.locator(".editor-layer").filter({ hasText: "固定文字" }).first();
    await fixedRow.getByRole("button", { name: "锁定", exact: true }).click();
    assert(await page.locator(".editor-properties-panel textarea").isDisabled());
    await fixedRow.getByRole("button",{name:"显示固定文字",exact:true}).click();
    assert.equal(await page.locator("svg").getByText("固定标语", { exact: true }).count(), 0);
    await fixedRow.getByRole("button",{name:"显示固定文字",exact:true}).click();
    await fixedRow.getByRole("button", { name: "解锁", exact: true }).click();
    await btn("下移固定文字").click();
    await btn("删除此图层").click();
    await page.locator(".image-editor").press("Meta+z");
    assert.equal(await page.locator("svg").getByText("固定标语", { exact: true }).count(), 1);
    pass("图层新增、隐藏、锁定、排序、删除撤销");
    await layer().click();
    const beforeLayer = record().draft.document.layers.find((l) => l.slot === "main_title");
    const fontSize = beforeLayer.fontSize;
    const control = page.locator("svg g[data-editor] rect").last();
    const box = await control.boundingBox();
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width / 2 + 10, box.y + box.height / 2 + 6, { steps: 4 });
    await page.mouse.up();
    await page.locator(".image-editor").press("Shift+ArrowRight");
    await save();
    const afterLayer = record().draft.document.layers.find((l) => l.slot === "main_title");
    assert(afterLayer.w > beforeLayer.w);
    assert.equal(afterLayer.fontSize, fontSize);
    assert.equal(afterLayer.x, beforeLayer.x + 10);
    pass("画布控制点调整区域不改变字号及键盘移动");
    await page.locator(".background-layer").click();
    await page.locator(".editor-properties-panel .t-select__wrap").first().click();
    await page.getByText("固定图片", { exact: true }).last().click();
    await page.getByText("固定背景图片", { exact: true }).waitFor();
    const bgName = snapshot.data.images.find((i) => i.enabled && i.uses?.includes("底图")).name;
    await page.locator(".editor-properties-panel .cover-field").filter({ hasText: "固定背景图片" }).locator(".t-select__wrap").click();
    await page.locator(".t-select-option:visible").filter({ hasText: bgName }).first().click();
    await page.locator(".editor-properties-panel .cover-field").filter({ hasText: "填充方式" }).locator(".t-select__wrap").click();
    await page.getByText("完整显示", { exact: true }).last().click();
    await page.getByText("固定背景图片", { exact: true }).waitFor();
    pass("背景连续编辑保持选中及裁切设置");
    await page.locator(".editor-properties-panel .t-select__wrap").first().click();
    await page.getByText("业务底图", { exact: true }).last().click();
    await footer().getByRole("button", { name: "预览与校验", exact: true }).click();
    await page.locator(".cover-preview-stage img").waitFor();
    await shot("08-business-background");
    await footer().getByRole("button", { name: "返回编辑", exact: true }).click();
    await page.getByRole("link", {name:"图片库",exact:true}).click();
    await btn("放弃修改").click();
    await page.getByRole("heading", { name: "图片库", exact: true }).waitFor();
    pass("业务底图预览及侧栏离开后到达原目标");
    await page.getByRole("link", {name:"封面模板",exact:true}).click();
    const legacy = page.getByRole("row").filter({ has: page.getByText("通用动态短标题 1200×900", { exact: true }) });
    await legacy.getByRole("button", { name: "编辑", exact: true }).click();
    await layer().click();
    await page.locator(".editor-properties-panel input[type=color]").first().fill("#ff0000");
    await page.locator(".editor-property-section summary").filter({hasText:"排版规则"}).click();
    await page.locator(".editor-properties-panel .editor-field").filter({ hasText: "最多字数" }).locator("input").fill("0");
    await page.locator(".editor-properties-panel .editor-field").filter({ hasText: "最多字数" }).locator("input").press("Tab");
    await save();
    await leaveEditor();
    await legacy.getByRole("button", { name: "发布更新", exact: true }).click();
    await page.getByText("校验通过，可以发布。", { exact: true }).waitFor();
    await btn("确认发布").click();
    await page.waitForTimeout(400);
    assert.equal(await legacy.locator(".cover-thumbnail img").getAttribute("src"), snapshot.data.templates.find((t) => t.name === "通用动态短标题 1200×900").preview);
    pass("存量模板发布更新后的缩略图刷新");
    const copiedRow = page.getByRole("row").filter({ has: page.getByText("V1 走查模板 副本", { exact: true }) });
    await copiedRow.getByRole("button", { name: "删除", exact: true }).click();
    await btn("取消").last().click();
    assert(snapshot.data.templates.some((t) => t.name === "V1 走查模板 副本"));
    await copiedRow.getByRole("button", { name: "删除", exact: true }).click();
    await btn("删除草稿").click();
    await page.waitForTimeout(300);
    assert(!snapshot.data.templates.some((t) => t.name === "V1 走查模板 副本"));
    pass("草稿删除确认与取消");
    await btn("新建模板").click();
    await page.getByPlaceholder("请输入模板名称").fill("创建失败保留");
    fail = true;
    await btn("创建并编辑").click();
    await page.getByText("操作失败，请重试", { exact: true }).waitFor();
    assert.equal(await page.getByPlaceholder("请输入模板名称").inputValue(), "创建失败保留");
    fail = false;
    await btn("取消").last().click();
    assert(!snapshot.data.templates.some((t) => t.name === "创建失败保留"));
    pass("创建失败保留表单及取消不建记录");
    await page.getByPlaceholder("搜索模板名称").fill("不存在的模板检索");
    await btn("查询").click();
    await page.getByText("没有符合条件的模板", { exact: true }).waitFor();
    await btn("重置筛选").click();
    await page.getByText("V1 走查模板", { exact: true }).waitFor();
    pass("查询空结果及重置");
    assert.deepEqual(errors, []);
    fs.writeFileSync(evidence + "/browser-results.json", JSON.stringify({ passed, errors, viewports: ["1440x900", "1280x800"], isolation: "Playwright route-backed snapshot; no user records changed" }, null, 2));
  } catch (e) {
    console.log("FAIL", e.stack);
    console.log((await page.locator("body").innerText()).slice(-6e3));
    await shot("failure");
    throw e;
  } finally {
    await browser.close();
  }
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
