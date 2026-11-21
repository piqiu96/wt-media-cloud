<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from "vue";
import MediaPreview from "./MediaPreview.vue";
import ResourceNotice from "../resource/ResourceNotice.vue";
import { BrowseIcon, BrowseOffIcon, LockOnIcon, LockOffIcon, ChevronUpIcon, ChevronDownIcon } from "tdesign-icons-vue-next";
import { layoutText } from "./textLayout.js";
const props = defineProps({ modelValue: Object, fixedSize: Boolean, templateMode: Boolean, backgroundPreview: String, fontRevision: Number, fontFamilyMap: { type: Object, default: () => ({}) } });
const emit = defineEmits(["update:modelValue", "export", "ready"]);
const base = () => ({
  width: 1200,
  height: 900,
  background: "#172f4c",
  image: "",
  layers: [
    {
      id: "title",
      kind: "text",
      name: "主标题",
      text: "精彩从这里开始",
      x: 80,
      y: 560,
      w: 1040,
      h: 250,
      fontSize: 90,
      minFont: 32,
      fontFamily: "sans-serif",
      weight: "700",
      color: "#ffffff",
      align: "left",
      lineHeight: 1.25,
      spacing: 0,
      maxLines: 2,
      maxChars: 30,
      autoShrink: true,
      wrap: true,
      stroke: 0,
      shadow: false,
      visible: true,
      locked: false,
    },
  ],
});
const doc = ref(base()),
  selected = ref(props.templateMode ? "background" : "title"),
  history = ref([]),
  future = ref([]),
  svg = ref(null),
  propertyPanel = ref(null),
  error = ref(""),
  resize = ref(1);
let external = false;
const clone = (x) => JSON.parse(JSON.stringify(x));
watch(
  () => props.modelValue,
  (v) => {
    if (v && !external) {
      doc.value = clone(v);
      history.value = [];
      future.value = [];
      selected.value = props.templateMode ? (selected.value === "background" || v.layers?.some(l=>l.id===selected.value) ? selected.value : null) : v.layers?.[0]?.id;
    }
  },
  { immediate: true },
);
const layer = computed(() =>
  doc.value.layers.find((l) => l.id === selected.value),
);
const ctx =
  typeof document !== "undefined"
    ? document.createElement("canvas").getContext("2d")
    : null;
function resolvedFont(family) { return props.fontFamilyMap[family] || family || "sans-serif"; }
function textLayout(l) {
  void props.fontRevision;
  return layoutText(l, (s, size) => {
    ctx.font = `${l.weight || 400} ${size}px ${JSON.stringify(resolvedFont(l.fontFamily))}`;
    return ctx.measureText(s).width;
  });
}
function layerProblem(l) {
 if (!props.templateMode || l.kind!=='text') return false;
 const safe=Number(doc.value.safe)||0;
 return textLayout(l).overflow || l.x<safe || l.y<safe || l.x+l.w>doc.value.width-safe || l.y+l.h>doc.value.height-safe;
}
const overflow = computed(() =>
  doc.value.layers.some(
    (l) => l.visible && l.kind === "text" && textLayout(l).overflow,
  ),
);
function publish() {
  external = true;
  emit("update:modelValue", clone(doc.value));
  queueMicrotask(() => (external = false));
}
function snapshot() {
  history.value.push(clone(doc.value));
  if (history.value.length > 40) history.value.shift();
  future.value = [];
}
function change(fn) {
  snapshot();
  fn();
  publish();
}
function patch(k, v) {
  if (!layer.value || layer.value.locked) return;
  change(() => {
    layer.value[k] = v;
    if (k === "fontSize") {
      layer.value.fontSize = Math.max(8, Number(v) || 8);
      layer.value.minFont = Math.min(layer.value.minFont || 8, layer.value.fontSize);
    }
    if (k === "minFont") {
      layer.value.minFont = Math.max(8, Math.min(layer.value.fontSize, Number(v) || 8));
    }
    if (["x", "y", "w", "h"].includes(k)) {
      const l = layer.value;
      l.w = Math.max(1, Math.min(doc.value.width, Number(l.w) || 1));
      l.h = Math.max(1, Math.min(doc.value.height, Number(l.h) || 1));
      l.x = Math.max(0, Math.min(doc.value.width - l.w, Number(l.x) || 0));
      l.y = Math.max(0, Math.min(doc.value.height - l.h, Number(l.y) || 0));
    }
  });
}
function undo() {
  if (!history.value.length) return;
  future.value.push(clone(doc.value));
  doc.value = history.value.pop();
  publish();
}
function redo() {
  if (!future.value.length) return;
  history.value.push(clone(doc.value));
  doc.value = future.value.pop();
  publish();
}
function add(kind, slot = "fixed") {
  if (slot === "main_title" && doc.value.layers.some(l => l.slot === "main_title")) return;
  change(() => {
    const id = crypto.randomUUID();
    doc.value.layers.push({
      id,
      kind,
      slot,
      name: slot === "main_title" ? "动态主标题" : kind === "text" ? "固定文字" : "固定装饰",
      text: "新的文字",
      x: 80,
      y: 80,
      w: 420,
      h: 160,
      fontSize: 52,
      minFont: 20,
      fontFamily: "sans-serif",
      weight: "700",
      color: kind === "text" ? "#ffffff" : "#3481ed",
      align: "left",
      lineHeight: 1.3,
      maxLines: 3,
      autoShrink: true,
      visible: true,
      locked: false,
    });
    selected.value = id;
  });
}
const guides = ref({x:false,y:false});
function pointer(e, l, mode) {
  selected.value = l.id;
  if (l.locked) return;
  snapshot();
  const rect = svg.value.getBoundingClientRect(),
    start = { x: e.clientX, y: e.clientY, l: clone(l) };
  e.target.setPointerCapture(e.pointerId);
  const move = (v) => {
    const dx = ((v.clientX - start.x) * doc.value.width) / rect.width,
      dy = ((v.clientY - start.y) * doc.value.height) / rect.height;
    if (mode === "size") {
      l.w = Math.max(30, Math.min(doc.value.width - l.x, start.l.w + dx));
      l.h = Math.max(30, Math.min(doc.value.height - l.y, start.l.h + dy));
    } else {
      l.x = Math.round(
        Math.max(0, Math.min(doc.value.width - l.w, start.l.x + dx)),
      );
      l.y = Math.round(
        Math.max(0, Math.min(doc.value.height - l.h, start.l.y + dy)),
      );
      if (props.templateMode) {
        guides.value={x:Math.abs(l.x+l.w/2-doc.value.width/2)<8,y:Math.abs(l.y+l.h/2-doc.value.height/2)<8};
        if(guides.value.x)l.x=Math.round((doc.value.width-l.w)/2);
        if(guides.value.y)l.y=Math.round((doc.value.height-l.h)/2);
      }
    }
  };
  const end = () => {
    guides.value={x:false,y:false};
    e.target.removeEventListener("pointermove", move);
    e.target.removeEventListener("pointerup", end);
    e.target.removeEventListener("pointercancel", end);
    publish();
  };
  e.target.addEventListener("pointermove", move);
  e.target.addEventListener("pointerup", end);
  e.target.addEventListener("pointercancel", end);
}
function upload(e) {
  const f = e.target.files?.[0];
  if (!f) return;
  if (!["image/png", "image/jpeg", "image/webp"].includes(f.type)) {
    error.value = "请选择 PNG、JPEG 或 WebP 图片";
    return;
  }
  const r = new FileReader();
  r.onload = () => change(() => (doc.value.image = r.result));
  r.readAsDataURL(f);
}
async function exportPng(download = true) {
  error.value = "";
  if (overflow.value) {
    error.value = "文字溢出，请调整内容或文字区域";
    return null;
  }
  try {
    await document.fonts.ready;
    const node = svg.value.cloneNode(true);
    node.removeAttribute("style");
    node.setAttribute("width", doc.value.width);
    node.setAttribute("height", doc.value.height);
    node.querySelectorAll("[data-editor]").forEach((n) => n.remove());
    const data =
      "data:image/svg+xml;charset=utf-8," +
      encodeURIComponent(new XMLSerializer().serializeToString(node));
    return await new Promise((resolve) => {
      const image = new Image();
      image.onload = () => {
        try {
          const c = document.createElement("canvas");
          c.width = doc.value.width * resize.value;
          c.height = doc.value.height * resize.value;
          c.getContext("2d").drawImage(image, 0, 0, c.width, c.height);
          const url = c.toDataURL("image/png");
          if (download) emit("export", url);
          if (download) {
            const a = document.createElement("a");
            a.href = url;
            a.download = "cover.png";
            a.click();
          }
          resolve(url);
        } catch {
          error.value = "图片导出失败，请更换本地底图后重试";
          resolve(null);
        }
      };
      image.onerror = () => {
        error.value = "图片无法导出";
        resolve(null);
      };
      image.src = data;
    });
  } catch {
    error.value = "字体或图片未准备完成";
    return null;
  }
}
function setBackground(src) {
  change(() => (doc.value.image = src));
}
function reorder(l, offset) {
  const i = doc.value.layers.indexOf(l),
    to = i + offset;
  if (to < 0 || to >= doc.value.layers.length) return;
  change(
    () =>
      ([doc.value.layers[i], doc.value.layers[to]] = [
        doc.value.layers[to],
        doc.value.layers[i],
      ]),
  );
}
const zoom = ref("fit"),
  previewOpen = ref(false),
  previewSrc = ref(""),
  rendering = ref(false),
  advanced = ref(false),
  fileInput = ref(null);
async function showPreview() {
  rendering.value = true;
  try {
    previewSrc.value = await exportPng(false);
    if (previewSrc.value) previewOpen.value = true;
  } finally {
    rendering.value = false;
  }
}
function removeLayer() {
  if (!layer.value || layer.value.locked) return;
  change(() => {
    doc.value.layers = doc.value.layers.filter((l) => l.id !== selected.value);
    selected.value = doc.value.layers.at(-1)?.id;
  });
}
function changeCanvas(value) {
  const [w, h] = value.split("x").map(Number);
  change(() => {
    const sx = w / doc.value.width,
      sy = h / doc.value.height;
    doc.value.layers.forEach((l) => {
      l.x = Math.round(l.x * sx);
      l.y = Math.round(l.y * sy);
      l.w = Math.round(l.w * sx);
      l.h = Math.round(l.h * sy);
      if (l.kind === "text") {
        l.fontSize = Math.round(l.fontSize * Math.min(sx, sy));
        l.minFont = Math.max(12, Math.round(l.minFont * Math.min(sx, sy)));
      }
    });
    doc.value.width = w;
    doc.value.height = h;
  });
}
function editorKey(e) {
  if (!e.target.closest("input,textarea,[contenteditable=true]")) {
    const steps={ArrowLeft:[-1,0],ArrowRight:[1,0],ArrowUp:[0,-1],ArrowDown:[0,1]};
    if (steps[e.key] && layer.value && !layer.value.locked) {e.preventDefault();const [x,y]=steps[e.key],n=e.shiftKey?10:1;change(()=>{layer.value.x+=x*n;layer.value.y+=y*n;});}
  }
  if (
    (e.ctrlKey || e.metaKey) &&
    e.key.toLowerCase() === "z" &&
    !e.target.closest("input,textarea,[contenteditable=true]")
  ) {
    e.preventDefault();
    e.shiftKey ? redo() : undo();
  }
}
function selectLayer(id, field) {
 selected.value = id;
 advanced.value = true;
 if (field) nextTick(() => {
  const target = propertyPanel.value?.querySelector(`[data-property="${field}"]`);
  if (!target) return;
  const section = target.closest("details");
  if (section) section.open = true;
  target.scrollIntoView({block:"nearest"});
  target.querySelector("input")?.focus({preventScroll:true});
 });
}
function duplicateLayer() {
 if (!layer.value || layer.value.locked || layer.value.slot === 'main_title') return;
 change(()=>{const next=clone(layer.value);next.id=crypto.randomUUID();next.name+=' 副本';next.x+=10;next.y+=10;doc.value.layers.push(next);selected.value=next.id;});
}
const showSafe = ref(true), collapsed = ref(false), editorGrid=ref(null), stageHeight=ref(null);
function fitWorkspace() {
 if (!props.templateMode || !editorGrid.value?.getClientRects().length) return;
 const footer=editorGrid.value.closest('.resource-detail-page')?.querySelector('.resource-detail-page__footer');
 stageHeight.value=Math.max(320,window.innerHeight-editorGrid.value.getBoundingClientRect().top-(footer?.getBoundingClientRect().height||69)-20);
}
watch([overflow,error],()=>nextTick(fitWorkspace));
let layoutObserver;
onBeforeUnmount(()=>{window.removeEventListener('resize',fitWorkspace);layoutObserver?.disconnect();});
defineExpose({ exportPng, hasOverflow: overflow, setBackground, showPreview, selectLayer });

onMounted(async () => {
  if(props.templateMode){
   await nextTick();fitWorkspace();
   window.addEventListener('resize',fitWorkspace);
   layoutObserver=new ResizeObserver(fitWorkspace);layoutObserver.observe(editorGrid.value.closest('.resource-overlay__body') || editorGrid.value.parentElement);
  }
  await document.fonts.ready;
  publish();
  emit("ready", clone(doc.value));
});
</script>
<template>
  <div class="image-editor" :class="{ 'template-editor-engine': templateMode, 'layers-collapsed': collapsed }" tabindex="0" @keydown="editorKey">
    <div class="editor-toolbar">
      <t-button v-if="templateMode" theme="default" variant="text" @click="collapsed = !collapsed">{{ collapsed ? "展开图层" : "折叠图层" }}</t-button>
      <div class="editor-history">
        <t-button
          theme="default"
          variant="outline"
          :disabled="!history.length"
          @click="undo"
          >撤销</t-button
        ><t-button
          theme="default"
          variant="outline"
          :disabled="!future.length"
          @click="redo"
          >重做</t-button
        >
      </div>
      <span v-if="fixedSize && !templateMode">{{ doc.width }} × {{ doc.height }} px</span
      ><t-select
        v-else-if="!fixedSize"
        :value="`${doc.width}x${doc.height}`"
        aria-label="画布尺寸"
        class="canvas-size-select"
        @change="changeCanvas"
        :options="[
          { label: '4:3 · 1200 × 900', value: '1200x900' },
          { label: '16:9 · 1920 × 1080', value: '1920x1080' },
          { label: '3:4 · 1080 × 1440', value: '1080x1440' },
          { label: '9:16 · 1080 × 1920', value: '1080x1920' },
          { label: '1:1 · 1080 × 1080', value: '1080x1080' },
        ]"
      />
      <t-select
        v-model="zoom"
        aria-label="画布缩放"
        class="canvas-zoom-select"
        :options="[
          { label: '适应画布', value: 'fit' },
          { label: '50%', value: '0.5' },
          { label: '75%', value: '0.75' },
          { label: '100%', value: '1' },
        ]"
      />
      <div v-if="!templateMode" class="editor-export">
        <t-button variant="outline" :loading="rendering" @click="showPreview"
          >成品预览</t-button
        ><t-button theme="default" variant="text" @click="exportPng(true)"
          >导出 PNG</t-button
        >
      </div>
    </div>
    <t-alert
      v-if="error || overflow"
      theme="warning"
      :message="error || '文字超出区域或字数限制，请在文字属性中调整。'"
      class="editor-warning"
    />
    <div ref="editorGrid" class="editor-grid" :style="templateMode && stageHeight ? {height:`${stageHeight}px`} : undefined">
      <aside class="editor-layers">
        <div class="editor-panel-title">
          <h3>图层</h3>
          <span>{{ doc.layers.length }}</span>
        </div>
        <div
          class="editor-layer"
          v-for="l in [...doc.layers].reverse()"
          :key="l.id"
          :class="{ 'is-selected': selected === l.id }"
        >
          <button
            class="layer-name"
            :aria-pressed="selected === l.id"
            @click="selected = l.id"
          >
            <span class="layer-kind">{{ l.kind === "text" ? "T" : "▰" }}</span><span class="layer-label" :title="l.name">{{ l.name }}<small v-if="templateMode">{{l.slot==='main_title'?'动态字段':l.kind==='text'?'固定文字':'装饰'}}</small></span>
          </button>
          <div v-if="templateMode" class="layer-actions compact-layer-actions">
            <t-button shape="square" theme="default" variant="text" :aria-label="`显示${l.name}`" :aria-pressed="l.visible" :title="l.visible?'隐藏图层':'显示图层'" @click="change(()=>l.visible=!l.visible)"><BrowseIcon v-if="l.visible"/><BrowseOffIcon v-else/></t-button>
            <t-button shape="square" theme="default" variant="text" :aria-label="l.locked?'解锁':'锁定'" :aria-pressed="l.locked" :title="l.locked?'解锁图层':'锁定图层'" @click="change(()=>l.locked=!l.locked)"><LockOnIcon v-if="l.locked"/><LockOffIcon v-else/></t-button>
            <t-button shape="square" theme="default" variant="text" :disabled="doc.layers.indexOf(l)===doc.layers.length-1" :aria-label="`上移${l.name}`" title="上移图层" @click="reorder(l,1)"><ChevronUpIcon/></t-button>
            <t-button shape="square" theme="default" variant="text" :disabled="doc.layers.indexOf(l)===0" :aria-label="`下移${l.name}`" title="下移图层" @click="reorder(l,-1)"><ChevronDownIcon/></t-button>
          </div>
          <div v-else class="layer-actions">
            <t-checkbox :checked="l.visible" @change="change(() => (l.visible = !l.visible))" :aria-label="`显示${l.name}`" />
            <t-button size="small" variant="text" @click="change(() => (l.locked = !l.locked))">{{ l.locked ? "解锁" : "锁定" }}</t-button>
            <t-button size="small" variant="text" :disabled="doc.layers.indexOf(l) === doc.layers.length - 1" @click="reorder(l, 1)" :aria-label="`上移${l.name}`">↑</t-button>
            <t-button size="small" variant="text" :disabled="doc.layers.indexOf(l) === 0" @click="reorder(l, -1)" :aria-label="`下移${l.name}`">↓</t-button>
          </div>
        </div>
        <div v-if="!doc.layers.length" class="editor-empty-layers">
          当前只有底图，可直接调整填充或添加文字。
        </div>
        <button v-if="templateMode" class="editor-layer layer-name background-layer" :class="{'is-selected': selected === 'background'}" :aria-pressed="selected==='background'" @click="selected = 'background'"><span class="layer-kind">▧</span><span class="layer-label">背景<small>固定底层</small></span></button>
        <div class="editor-add">
          <t-button v-if="templateMode && !doc.layers.some(l=>l.slot==='main_title')" theme="default" variant="outline" @click="add('text','main_title')">动态主标题</t-button>
          <t-button theme="default" variant="outline" @click="add('text')"
            >{{ templateMode ? "固定文字" : "添加文字" }}</t-button
          ><t-button
            theme="default"
            variant="outline"
            @click="add('decoration')"
            >添加装饰</t-button
          >
        </div>
        <div v-if="!templateMode" class="editor-background">
          <h3>背景</h3>
          <div class="editor-field">
            背景颜色<input
              type="color"
              :value="doc.background"
              @change="change(() => (doc.background = $event.target.value))"
              aria-label="背景颜色"
            />
          </div>
          <input
            ref="fileInput"
            class="editor-file-input"
            type="file"
            accept="image/png,image/jpeg,image/webp"
            @change="upload"
          /><t-button
            theme="default"
            variant="outline"
            @click="fileInput.click()"
            >上传底图</t-button
          ><template v-if="doc.image"
            ><div class="editor-field">
              填充方式<t-select
                :value="doc.imageFit || 'slice'"
                @change="(v) => change(() => (doc.imageFit = v))"
                :options="[
                  { label: '填满并裁切', value: 'slice' },
                  { label: '完整显示', value: 'meet' },
                ]"
              />
            </div>
            <div class="editor-field">
              图片定位<t-select
                :value="doc.imageAnchor || 'xMidYMid'"
                @change="(v) => change(() => (doc.imageAnchor = v))"
                :options="[
                  { label: '居中', value: 'xMidYMid' },
                  { label: '靠左', value: 'xMinYMid' },
                  { label: '靠右', value: 'xMaxYMid' },
                  { label: '靠上', value: 'xMidYMin' },
                  { label: '靠下', value: 'xMidYMax' },
                ]"
              />
            </div>
            <t-button
              theme="default"
              variant="text"
              @click="change(() => (doc.image = ''))"
              >移除底图</t-button
            ></template
          >
        </div>
      </aside>
      <div class="editor-stage">
        <div
          class="editor-canvas-viewport"
          :class="{ 'is-zoomed': zoom !== 'fit' }"
        >
          <svg
            @pointerdown.self="selected = null"
            ref="svg"
            :style="
              zoom === 'fit'
                ? {}
                : {
                    width: `${doc.width * Number(zoom)}px`,
                    height: `${doc.height * Number(zoom)}px`,
                    maxWidth: 'none',
                    maxHeight: 'none',
                  }
            "
            xmlns="http://www.w3.org/2000/svg"
            :viewBox="`0 0 ${doc.width} ${doc.height}`"
            :width="doc.width"
            :height="doc.height"
            role="img"
            aria-label="封面编辑画布"
          >
            <rect
              :width="doc.width"
              :height="doc.height"
              :fill="doc.background"
              @pointerdown="templateMode && (selected='background')"
            />
            <image
              v-if="backgroundPreview ?? doc.image"
              :href="backgroundPreview ?? doc.image"
              :width="doc.width"
              :height="doc.height"
              :preserveAspectRatio="`${doc.imageAnchor || 'xMidYMid'} ${doc.imageFit || 'slice'}`"
              @pointerdown="templateMode && (selected='background')"
            />
            <rect v-if="templateMode && showSafe" data-editor :x="doc.safe" :y="doc.safe" :width="doc.width - doc.safe * 2" :height="doc.height - doc.safe * 2" fill="none" stroke="#f5b553" stroke-dasharray="10 8" :stroke-width="2" pointer-events="none" />
            <g
              v-for="l in doc.layers.filter((l) => l.visible)"
              :key="l.id"
              @pointerdown.prevent="pointer($event, l, 'move')"
              @dblclick="!l.locked && l.kind === 'text' && $el.querySelector('.editor-properties-panel textarea')?.focus()"
              :style="{ cursor: l.locked ? 'default' : 'move' }"
            >
              <rect
                v-if="l.kind === 'decoration'"
                :x="l.x"
                :y="l.y"
                :width="l.w"
                :height="l.h"
                :fill="l.color"
                :opacity="l.opacity ?? 1"
              />
              <text
                v-else
                :font-family="resolvedFont(l.fontFamily)"
                :font-size="textLayout(l).size"
                :font-weight="l.weight"
                :fill="l.color"
                :stroke="l.stroke ? l.strokeColor || '#000' : undefined"
                :stroke-width="l.stroke || undefined"
                paint-order="stroke"
                :letter-spacing="l.spacing || 0"
                :style="{
                  filter: l.shadow
                    ? 'drop-shadow(3px 3px 4px #0009)'
                    : undefined,
                }"
              >
                <tspan
                  v-for="(line, i) in textLayout(l).lines"
                  :key="i"
                  :x="
                    l.x +
                    (l.align === 'center'
                      ? l.w / 2
                      : l.align === 'right'
                        ? l.w
                        : 0)
                  "
                  :y="l.y + textLayout(l).size + i * textLayout(l).lineHeight"
                  :text-anchor="
                    l.align === 'center'
                      ? 'middle'
                      : l.align === 'right'
                        ? 'end'
                        : 'start'
                  "
                >
                  {{ line }}
                </tspan>
              </text>
              <rect v-if="templateMode && selected!==l.id && layerProblem(l)" data-editor :x="l.x" :y="l.y" :width="l.w" :height="l.h" fill="none" stroke="var(--wt-danger)" stroke-width="3" stroke-dasharray="8 6" pointer-events="none" />
              <g v-if="selected === l.id" data-editor>
                <rect
                  :x="l.x"
                  :y="l.y"
                  :width="l.w"
                  :height="l.h"
                  fill="transparent"
                  :stroke="layerProblem(l)?'var(--wt-danger)':'#3c8cff'"
                  :stroke-width="3"
                />
                <rect
                  v-if="!l.locked"
                  :x="l.x + l.w - 15"
                  :y="l.y + l.h - 15"
                  width="30"
                  height="30"
                  fill="#3c8cff"
                  @pointerdown.stop.prevent="pointer($event, l, 'size')"
                  style="cursor: nwse-resize"
                />
              </g>
            </g>
            <path v-if="templateMode && guides.x" data-editor :d="`M ${doc.width/2} 0 V ${doc.height}`" stroke="#3c8cff" stroke-width="2" stroke-dasharray="8 6" pointer-events="none" />
            <path v-if="templateMode && guides.y" data-editor :d="`M 0 ${doc.height/2} H ${doc.width}`" stroke="#3c8cff" stroke-width="2" stroke-dasharray="8 6" pointer-events="none" />
          </svg>
        </div>
        <div class="editor-stage-caption">
          <span>{{ doc.width }} × {{ doc.height }} px</span
          ><span>拖动图层 · 右下角调整区域</span>
        </div>
      </div>
      <aside ref="propertyPanel" class="editor-properties-panel">
        <template v-if="layer">
          <div class="editor-panel-title">
            <h3>{{ layer.slot === "main_title" ? "动态主标题" : layer.kind === "text" ? "固定文字" : "装饰元素" }}</h3>
            <t-tag v-if="layer.locked" size="small">已锁定</t-tag>
          </div>
          <ResourceNotice v-if="layer.locked"
            >在左侧解锁后可调整此图层。</ResourceNotice
          >
          <fieldset :disabled="layer.locked">
            <h4 v-if="templateMode" class="editor-property-heading">内容</h4>
            <div v-if="templateMode && layer.slot==='main_title'" class="editor-binding">绑定字段<span>封面标题</span></div>
            <t-button v-if="templateMode && layer.slot==='sub_title'" theme="default" variant="outline" :disabled="layer.locked" @click="patch('slot','fixed')">转为固定文字</t-button>
            <label v-if="layer.kind==='text'" class="editor-field">{{templateMode && layer.slot==='main_title'?'预览样例':'文字内容'}}<t-textarea :value="layer.text" :disabled="layer.locked" :autosize="{minRows:2,maxRows:4}" @change="v=>patch('text',v)" /><small v-if="templateMode && layer.slot==='main_title'" class="editor-property-hint">正式生成时替换为业务封面标题</small></label>
            <h4 v-if="templateMode" class="editor-property-heading">样式</h4>
            <template v-if="layer.kind==='text'">
             <label class="editor-field">字体<t-select :value="layer.fontFamily" :disabled="layer.locked" @change="v=>patch('fontFamily',v)" :options="[{label:'系统黑体',value:'sans-serif'},{label:'系统宋体',value:'serif'},{label:'等宽字体',value:'monospace'}]" /></label>
             <div class="editor-property-pair">
              <label class="editor-field">字号<t-input-number :theme="templateMode ? 'column' : 'row'" :value="layer.fontSize" :min="8" :max="600" :disabled="layer.locked" @change="v=>patch('fontSize',v)" /></label>
              <label class="editor-field">字重<t-select :value="String(layer.weight)" :disabled="layer.locked" @change="v=>patch('weight',v)" :options="[{label:'常规',value:'400'},{label:'加粗',value:'700'}]" /></label>
             </div>
             <label class="editor-field">对齐<t-radio-group :value="layer.align" :disabled="layer.locked" :variant="templateMode ? 'outline' : 'default-filled'" @change="v=>patch('align',v)"><t-radio-button value="left">左</t-radio-button><t-radio-button value="center">中</t-radio-button><t-radio-button value="right">右</t-radio-button></t-radio-group></label>
            </template>
            <label class="editor-field editor-color-field">{{layer.kind==='text'?'文字颜色':'装饰颜色'}}<input type="color" :value="layer.color" :disabled="layer.locked" @change="patch('color',$event.target.value)"/><span>{{layer.color}}</span></label>
            <label v-if="templateMode && layer.kind==='decoration'" class="editor-field">透明度<t-input-number :theme="templateMode ? 'column' : 'row'" :value="layer.opacity ?? 1" :min="0" :max="1" :step="0.1" :disabled="layer.locked" @change="v=>patch('opacity',v)" /></label>
            <t-button v-if="!templateMode" theme="default" variant="text" class="advanced-properties-toggle" @click="advanced=!advanced">{{advanced?'收起':'展开'}}精确位置与排版</t-button>
            <template v-if="templateMode || advanced">
             <h4 v-if="templateMode" class="editor-property-heading">位置与尺寸</h4>
             <div class="editor-property-pair">
              <label v-for="[key,name] in [['x','X 坐标'],['y','Y 坐标'],['w','区域宽度'],['h','区域高度']]" :key="key" class="editor-field">{{name}}<t-input-number :theme="templateMode ? 'column' : 'row'" :value="layer[key]" :min="0" :disabled="layer.locked" @change="v=>patch(key,v)" /></label>
             </div>
             <template v-if="layer.kind==='text'">
              <component :is="templateMode?'details':'div'" class="editor-property-section">
               <summary v-if="templateMode">排版规则</summary>
               <div class="editor-property-pair">
                <label v-for="[key,name,min,step] in [['minFont','最小字号',8,1],['maxLines','最多行数',1,1],['maxChars','最多字数',0,1],['lineHeight','行高',1,0.1],['spacing','字间距',0,0.5]]" :key="key" :data-property="key" class="editor-field">{{name}}<t-input-number :theme="templateMode ? 'column' : 'row'" :value="layer[key] || 0" :min="min" :step="step" :disabled="layer.locked" @change="v=>patch(key,v)" /><small v-if="templateMode && key==='maxChars'" class="editor-property-hint">0 表示不限字数</small></label>
               </div>
               <t-checkbox v-for="[key,name] in [['autoShrink','自动缩小字号'],['wrap','自动换行']]" :key="key" :checked="layer[key]" :disabled="layer.locked" @change="v=>patch(key,v)">{{name}}</t-checkbox>
              </component>
              <component :is="templateMode?'details':'div'" class="editor-property-section">
               <summary v-if="templateMode">文字效果</summary>
               <label class="editor-field">描边宽度<t-input-number :theme="templateMode ? 'column' : 'row'" :value="layer.stroke || 0" :min="0" :disabled="layer.locked" @change="v=>patch('stroke',v)" /></label>
               <label class="editor-field">描边颜色<input type="color" :value="layer.strokeColor || '#000000'" :disabled="layer.locked" @change="patch('strokeColor',$event.target.value)" /></label>
               <t-checkbox :checked="layer.shadow" :disabled="layer.locked" @change="v=>patch('shadow',v)">文字阴影</t-checkbox>
              </component>
             </template>
             <details v-if="templateMode" class="editor-property-section"><summary>图层名称</summary><t-input :value="layer.name" :disabled="layer.locked" @change="v=>patch('name',v)" /></details>
            </template>
          </fieldset>
          <div class="editor-property-actions">
            <t-button v-if="templateMode && layer.slot !== 'main_title'" theme="default" variant="text" :disabled="layer.locked" @click="duplicateLayer">复制图层</t-button>
            <t-button
              theme="danger"
              variant="text"
              :disabled="layer.locked"
              @click="removeLayer"
              >删除此图层</t-button
            >
          </div>
        </template>
        <slot v-else-if="templateMode" name="settings" :selected="selected" :show-safe="showSafe" :set-show-safe="v=>showSafe=v" />
        <div v-else class="editor-empty-layers">
          选择左侧图层，或添加文字后开始排版。
        </div>
      </aside>
    </div>
    <MediaPreview
      v-model:visible="previewOpen"
      title="封面成品预览"
      :images="[
        { src: previewSrc, label: `${doc.width} × ${doc.height} · 当前成品` },
      ]"
    />
  </div>
</template>
<style scoped>
.image-editor {
  color: var(--wt-text-primary);
  font-size: 13px;
}
.editor-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--wt-border);
}
.editor-history {
  display: flex;
  gap: 8px;
}
.canvas-size-select {
  width: 205px;
}
.canvas-zoom-select {
  width: 125px;
}
.editor-export {
  display: flex;
  gap: 8px;
  margin-left: auto;
}
.editor-warning {
  margin-top: 16px;
}
.editor-grid {
  display: grid;
  grid-template-columns: 195px minmax(260px, 1fr) 252px;
  gap: 20px;
  margin-top: 20px;
  align-items: stretch;
}
.editor-grid aside {
  height: 490px;
  overflow: auto;
  padding-right: 8px;
  box-sizing: border-box;
  scrollbar-width: thin;
}
.editor-panel-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.editor-panel-title span {
  font-size: 12px;
  color: var(--wt-text-tertiary);
}
.editor-grid h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}
.editor-layer {
  border: 1px solid var(--wt-border);
  border-radius: 6px;
  margin-bottom: 10px;
  overflow: hidden;
}
.editor-layer.is-selected {
  border-color: var(--wt-primary);
  background: var(--wt-info-bg);
}
.layer-name {
  display: flex;
  gap: 10px;
  align-items: center;
  width: 100%;
  padding: 10px;
  border: 0;
  background: transparent;
  text-align: left;
  color: var(--wt-text-primary);
  cursor: pointer;
  font-size: 13px;
}
.layer-kind {
  font-size: 16px;
  font-weight: 600;
  color: var(--wt-text-tertiary);
}
.layer-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 0 8px 6px;
}
.layer-actions > .t-button {
  padding: 0 5px;
  min-width: 25px;
}
.layer-actions > .t-checkbox {
  margin-right: auto;
}
.editor-add {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 16px;
}
.editor-add > .t-button {
  flex: 1;
  padding: 0 8px;
}
.editor-background {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-top: 24px;
  border-top: 1px solid var(--wt-border);
  padding-top: 20px;
}
.image-editor .editor-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
  color: var(--wt-text-secondary);
}
.image-editor input[type="color"] {
  width: 100%;
  height: 32px;
  border: 1px solid var(--wt-border);
  padding: 3px;
  background: var(--wt-bg-card);
  border-radius: 4px;
}
.editor-file-input {
  display: none;
}
.editor-stage {
  min-width: 0;
  background: var(--wt-bg-page);
  border: 1px solid var(--wt-border);
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  height: 490px;
  box-sizing: border-box;
}
.editor-canvas-viewport {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 24px;
}
.editor-canvas-viewport svg {
  display: block;
  width: auto;
  max-width: 100%;
  height: auto;
  max-height: 100%;
  box-shadow: var(--wt-shadow-card);
  flex-shrink: 0;
  touch-action: none;
}
.editor-canvas-viewport.is-zoomed {
  display: block;
}
.editor-stage-caption {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  font-size: 11px;
  color: var(--wt-text-tertiary);
  flex-wrap: wrap;
  border-top: 1px solid var(--wt-border);
}
.image-editor fieldset {
  border: 0;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
.editor-property-pair {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  min-width: 0;
}
.editor-property-pair .editor-field {
  min-width: 0;
}
.editor-property-pair :deep(.t-input-number) {
  width: 100%;
  min-width: 0;
}
.editor-color-field span {
  font-size: 11px;
  color: var(--wt-text-tertiary);
}
.advanced-properties-toggle {
  align-self: flex-start;
  padding-left: 0;
  font-size: 12px;
}
.editor-empty-layers {
  font-size: 12px;
  color: var(--wt-text-tertiary);
  line-height: 1.8;
  padding: 12px 0;
}
@media (max-height: 850px) {
  .editor-grid aside,
  .editor-stage {
    height: 410px;
  }
}
@media (max-width: 1050px) {
  .editor-grid {
    grid-template-columns: 165px minmax(230px, 1fr) 220px;
    gap: 14px;
  }
  .editor-canvas-viewport {
    padding: 16px;
  }
}
@media (max-width: 760px) {
  .editor-grid {
    grid-template-columns: 145px minmax(0, 1fr);
  }
  .editor-properties-panel {
    grid-column: 1/-1;
    height: auto !important;
    max-height: 350px;
    border-top: 1px solid var(--wt-border);
    padding-top: 16px;
  }
  .editor-properties-panel fieldset {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
  .editor-export {
    margin-left: 0;
  }
  .editor-stage,
  .editor-layers {
    height: 350px !important;
  }
  .editor-stage-caption {
    font-size: 10px;
  }
}
@media (max-width: 480px) {
  .editor-grid {
    display: flex;
    flex-direction: column;
  }
  .editor-stage {
    order: 0;
  }
  .editor-layers {
    order: 1;
    height: auto !important;
    max-height: none;
  }
  .editor-properties-panel {
    order: 2;
    max-height: none;
  }
  .editor-properties-panel fieldset {
    display: flex;
  }
  .editor-toolbar {
    gap: 8px;
  }
  .canvas-size-select {
    width: 190px;
  }
  .editor-stage {
    height: 320px !important;
  }
  .editor-export {
    width: 100%;
  }
}

.template-editor-engine { outline:none; }
.template-editor-engine .editor-grid {grid-template-columns:220px minmax(260px,1fr) 280px;gap:0;margin-top:0;height:max(380px,calc(100dvh - 294px));box-sizing:border-box;border:1px solid var(--wt-border);border-radius:4px;overflow:hidden;background:var(--wt-bg-card);}
.template-editor-engine .editor-grid aside {height:100%;padding:16px 12px;min-width:0;}
.template-editor-engine .editor-layers{border-right:1px solid var(--wt-border);}
.template-editor-engine .editor-properties-panel{border-left:1px solid var(--wt-border);}
.template-editor-engine .editor-stage{min-height:0;height:100%;border:0;border-radius:0;background:var(--wt-bg-page);}
.template-editor-engine .editor-canvas-viewport{min-height:0;box-sizing:border-box;padding:24px;}
.template-editor-engine .editor-stage-caption{padding:8px 12px;font-size:12px;line-height:20px;flex-wrap:nowrap;}
.template-editor-engine .editor-stage-caption span:last-child{font-size:11px;}
.template-editor-engine .editor-toolbar{padding:0 0 12px;gap:8px;border:0;}
.template-editor-engine .canvas-zoom-select{margin-left:auto;width:140px;}
.template-editor-engine .editor-warning{margin:0 0 12px;}
.template-editor-engine .editor-panel-title{margin-bottom:12px;}
.template-editor-engine .editor-layer{display:flex;align-items:center;gap:2px;border:1px solid transparent;border-radius:4px;margin-bottom:4px;min-height:52px;}
.template-editor-engine .editor-layer:hover{background:var(--wt-color-bg-hover);}
.template-editor-engine .editor-layer.is-selected{border-color:var(--wt-primary);background:var(--wt-info-bg);}
.template-editor-engine .layer-name{padding:6px 4px;gap:6px;min-width:0;flex:1;}
.template-editor-engine .layer-kind{font-size:14px;flex-shrink:0;}
.template-editor-engine .layer-label{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;line-height:20px;font-size:13px;}
.template-editor-engine .layer-label small{display:block;color:var(--wt-text-tertiary);font-size:12px;line-height:16px;font-weight:400;}
.template-editor-engine .compact-layer-actions{padding:0 2px 0 0;gap:0;flex-shrink:0;}
.template-editor-engine .compact-layer-actions>.t-button{width:22px;min-width:22px;height:28px;padding:0;}
.template-editor-engine .compact-layer-actions :deep(.t-icon){font-size:14px;}
.template-editor-engine .background-layer{padding:6px 8px;width:100%;box-sizing:border-box;}
.template-editor-engine .editor-add{gap:8px;margin-top:16px;}
.template-editor-engine .editor-add>.t-button{flex:1 1 80px;font-size:13px;}
.template-editor-engine .editor-field{font-size:13px;gap:6px;}
.template-editor-engine fieldset{gap:12px;}
.template-editor-engine .editor-property-pair{gap:12px;}
.template-editor-engine .editor-property-actions{display:flex;flex-wrap:wrap;gap:8px;margin-top:12px;}
.template-editor-engine .editor-property-heading{margin:4px 0 0;padding-top:12px;border-top:1px solid var(--wt-border);font-size:13px;font-weight:500;color:var(--wt-text-primary);}
.template-editor-engine .editor-property-heading:first-child{padding-top:0;border:0;}
.template-editor-engine .editor-binding{display:flex;justify-content:space-between;color:var(--wt-text-tertiary);font-size:12px;}
.template-editor-engine .editor-binding span{color:var(--wt-text-secondary);}
.template-editor-engine .editor-property-hint{font-size:12px;line-height:18px;color:var(--wt-text-tertiary);}
.editor-property-section summary{cursor:pointer;font-size:13px;font-weight:500;color:var(--wt-text-primary);padding:8px 0;}
.editor-property-section>.editor-field,.editor-property-section>.editor-property-pair{margin:8px 0 12px;}
.editor-property-section>.t-checkbox{margin:4px 12px 4px 0;}
.template-editor-engine.layers-collapsed .editor-grid{grid-template-columns:minmax(300px,1fr) 280px;}
.template-editor-engine.layers-collapsed .editor-layers{display:none;}
@media(max-width:1280px){.template-editor-engine .editor-grid{grid-template-columns:210px minmax(220px,1fr) 260px;}.template-editor-engine.layers-collapsed .editor-grid{grid-template-columns:minmax(300px,1fr) 260px;}.template-editor-engine .editor-canvas-viewport{padding:20px;}.template-editor-engine .editor-stage-caption span:last-child{display:none;}}
@media(max-width:1000px){.template-editor-engine .editor-grid{grid-template-columns:190px minmax(220px,1fr) 250px;}.template-editor-engine .editor-grid aside{padding:12px 8px;}.template-editor-engine .compact-layer-actions>.t-button{width:20px;min-width:20px;}}
@media(max-width:760px){
 .template-editor-engine .editor-grid,.template-editor-engine.layers-collapsed .editor-grid{grid-template-columns:minmax(0,1fr);height:auto;gap:16px;overflow:visible;}
 .template-editor-engine .editor-grid aside{height:auto;max-height:none;}
 .template-editor-engine .editor-stage{height:340px!important;min-width:0;order:0;}
 .template-editor-engine .editor-layers{order:1;}.template-editor-engine .editor-properties-panel{order:2;}
}

</style>
