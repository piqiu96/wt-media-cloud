<script setup>
import { computed, onMounted, ref, watch } from "vue"
// Static import, as in `AccountsPage.vue`: a dynamic import() of a bare
// specifier does not resolve in the packaged Tauri WebView ("Module name ...
// does not resolve to a valid file"). It is inert in Cloud Web, which never
// reaches the code that calls it.
import { invoke } from "@tauri-apps/api/core"
import {
  createLocalSettingsService,
  DEFAULT_TAIL_LINES,
} from "../local-settings/service.js"
// `formatModified` and `logKindLabel` both live in the settings view module:
// they describe a stored setting and a listed file, and the log viewer renders
// both. They were looked for in `local-logs-view.js` first, which the unit suite
// cannot notice — it never compiles a `.vue` file, so a wrong specifier is a
// build error and nothing else. `localSettingsWiring.test.js` now checks every
// named import of every desktop page against the module's real exports.
import { formatModified, logKindLabel } from "../local-settings/local-settings-view.js"
import { createLocalLogsPage, NO_LEVEL } from "./local-logs-view.js"

// The whole viewer goes through the Rust bridge. It does **not** fetch the
// Agent's loopback port: the window's CSP blocks that, so the only message a
// direct request could ever produce was 「Agent 不可达」, whether an Agent was
// running or not. `localAgentBoundary.test.js` is what keeps that true.
const service = createLocalSettingsService({ invoke })

const loading = ref(true)
const busy = ref("")
const listing = ref(null)
const source = ref("")
const name = ref("")
const tail = ref(null)
const level = ref(NO_LEVEL)
const failure = ref("")
const openedPath = ref("")

const page = computed(() =>
  createLocalLogsPage({
    listing: listing.value,
    source: source.value,
    name: name.value,
    tail: tail.value,
    level: level.value,
  })
)

// The two trees, in the order the listing returned them, with a heading per
// tree. The directory is shown per tree rather than per file: it is the answer
// to 「为什么这里是空的」, and repeating it on every row would bury it.
const treeHeadings = computed(() =>
  page.value.trees.map((tree) => ({
    source: tree.source,
    title: tree.source === "desktop" ? "本机控制台日志" : "本机执行服务日志",
    directory: tree.directory,
    files: tree.files,
    isEmpty: tree.files.length === 0,
  }))
)

async function run(key, work) {
  busy.value = key
  failure.value = ""
  try {
    return await work()
  } catch (error) {
    failure.value = error?.message || String(error) || "操作失败"
    return null
  } finally {
    busy.value = ""
  }
}

async function reloadListing() {
  const next = await run("listing", () => service.logFiles())
  if (!next) {
    return false
  }
  listing.value = next

  // Keep the selection if it is still there; otherwise land on the first file
  // the listing offers. A viewer that opened with nothing selected would make a
  // person click twice to see anything, and one that kept a vanished selection
  // would show a stale tail under a file that is no longer listed.
  const stillThere = next.trees.some(
    (tree) => tree.source === source.value && tree.files.some((file) => file.name === name.value)
  )
  if (!stillThere) {
    const first = next.trees.find((tree) => tree.files.length > 0)
    source.value = first?.source ?? ""
    name.value = first?.files[0]?.name ?? ""
    tail.value = null
  }
  return true
}

async function reloadTail() {
  if (!source.value || !name.value) {
    tail.value = null
    return
  }
  // The filter is sent as the wire's `minLevel`, and `null` means 「no filter」 —
  // not `""`, which the Rust side would parse as an unknown level and refuse.
  const next = await run("tail", () =>
    service.logTail({
      source: source.value,
      name: name.value,
      lines: DEFAULT_TAIL_LINES,
      minLevel: level.value,
    })
  )
  if (next) {
    tail.value = next
  }
}

async function reloadAll() {
  loading.value = true
  try {
    const ok = await reloadListing()
    if (ok) {
      await reloadTail()
    }
  } finally {
    loading.value = false
  }
}

// Selecting a file shows it immediately at the current filter: the filter is a
// property of the viewer, not of the file, so switching files must not silently
// drop it.
async function selectFile(nextSource, nextName) {
  source.value = nextSource
  name.value = nextName
  await reloadTail()
}

// Changing the filter re-reads rather than re-filtering what is already on
// screen. The filtering belongs to the command — it is the only side that knows
// which lines are unclassifiable — and a second filter here could disagree with
// the count the command reported.
watch(level, () => {
  void reloadTail()
})

async function openFolder() {
  const place = source.value
  if (!place) {
    return
  }
  // The label handed over is the tree's own `source` — literally the word
  // `local_log_tail` takes. One vocabulary, so the folder that opens is the one
  // being read.
  const opened = await run("open", () => service.openPlace(place))
  if (opened) {
    openedPath.value = opened
  }
}

onMounted(reloadAll)
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <div class="logs-page">
      <t-card title="本地日志" :bordered="true">
        <template #description>
          两侧日志树里的实际文件。左侧是磁盘上的文件，右侧是它尾部的内容。
        </template>

        <div class="toolbar">
          <t-select
            v-model="level"
            :options="page.filterOptions"
            :disabled="!!busy"
            class="level-select"
            placeholder="级别筛选"
          />
          <t-button theme="default" :loading="busy === 'listing'" :disabled="!!busy" @click="reloadAll">
            刷新
          </t-button>
          <t-button
            v-if="page.canView"
            theme="default"
            :loading="busy === 'open'"
            :disabled="!!busy"
            @click="openFolder"
          >
            打开日志文件夹
          </t-button>
        </div>

        <t-alert
          v-if="page.appliedFilterText !== '未筛选'"
          theme="info"
          class="mt-sm"
          :message="`当前筛选：${page.appliedFilterText}（该级别及以上，未分级的行不受影响）`"
        />
      </t-card>

      <div class="split">
        <t-card title="日志文件" :bordered="true" class="pane">
          <template #description>按来源分组；正在写入的文件永远在最前</template>
          <t-empty v-if="!page.hasTrees" description="未读到日志目录" />
          <div v-for="tree in treeHeadings" :key="tree.source" class="tree">
            <div class="tree-head">
              <span class="tree-title">{{ tree.title }}</span>
              <span class="tree-dir">{{ tree.directory }}</span>
            </div>
            <t-empty v-if="tree.isEmpty" size="small" description="暂无文件" />
            <ul v-else class="file-list">
              <li
                v-for="file in tree.files"
                :key="`${tree.source}/${file.name}`"
                class="file-row"
                :class="{ selected: file.selected }"
                @click="selectFile(tree.source, file.name)"
              >
                <span class="file-name">{{ file.name }}</span>
                <span class="file-meta">
                  <t-tag size="small" variant="light-outline">{{ logKindLabel(file.kind) }}</t-tag>
                  <span class="muted">{{ file.bytes }} B</span>
                  <span class="muted">{{ formatModified(file.modifiedSeconds) }}</span>
                </span>
              </li>
            </ul>
          </div>
        </t-card>

        <t-card :bordered="true" class="pane viewer">
          <template #description>
            {{ page.canView ? page.selectedPath : "先在左侧选择一个文件" }}
          </template>
          <template #title>
            {{ page.selectedName || "日志内容" }}
          </template>

          <div v-if="page.canView" class="meta">
            <span>{{ page.countsText }}</span>
            <span v-if="page.selectedDirectory" class="muted">{{ page.selectedDirectory }}</span>
          </div>

          <t-alert
            v-if="page.hiddenNotice"
            theme="info"
            class="mt-sm"
            :message="page.hiddenNotice"
          />
          <t-alert
            v-if="page.truncatedNotice"
            theme="warning"
            class="mt-sm"
            :message="page.truncatedNotice"
          />

          <t-empty v-if="page.canView && !page.hasLines" description="没有符合当前筛选的行" />
          <ol v-else-if="page.hasLines" class="lines">
            <li
              v-for="line in page.lines"
              :key="line.key"
              class="line"
              :class="{ continues: line.indent }"
            >
              <span class="gutter">
                <t-tag v-if="line.levelText" size="small" :theme="line.levelTheme" variant="light">
                  {{ line.levelText }}
                </t-tag>
              </span>
              <span class="text">{{ line.text }}</span>
            </li>
          </ol>
        </t-card>
      </div>

      <t-alert v-if="openedPath" theme="success" class="mt-sm" :message="`已打开 ${openedPath}`" />
      <t-alert v-if="failure" theme="error" class="mt-sm" :message="failure" />
    </div>
  </t-loading>
</template>

<style scoped>
.logs-page {
  margin: 16px;
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}

.level-select {
  width: 200px;
}

.mt-sm {
  margin-top: 12px;
}

.split {
  display: grid;
  grid-template-columns: minmax(280px, 380px) 1fr;
  gap: 16px;
  margin-top: 16px;
}

@media (max-width: 1100px) {
  .split {
    grid-template-columns: 1fr;
  }
}

.pane {
  min-width: 0;
}

.tree + .tree {
  margin-top: 16px;
}

.tree-head {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-bottom: 8px;
}

.tree-title {
  font-weight: 600;
}

.tree-dir {
  color: #8a8a8a;
  font-size: 12px;
  word-break: break-all;
}

.file-list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.file-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px;
  border-radius: 4px;
  cursor: pointer;
}

.file-row:hover {
  background: rgba(30, 64, 175, 0.04);
}

.file-row.selected {
  background: rgba(30, 64, 175, 0.06);
  border-left: 3px solid #2563eb;
}

.file-name {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  word-break: break-all;
}

.file-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  font-size: 12px;
}

.muted {
  color: #8a8a8a;
}

.viewer {
  display: flex;
  flex-direction: column;
}

.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  font-size: 12px;
  color: #4a4a4a;
}

.lines {
  margin: 12px 0 0;
  padding: 0;
  max-height: 60vh;
  overflow: auto;
  background: #fbfbf9;
  border: 1px solid #ededed;
  border-radius: 4px;
  list-style: none;
}

.line {
  display: flex;
  gap: 8px;
  padding: 2px 8px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.7;
}

/* A continuation is the record above it, not a record of its own. */
.line.continues {
  padding-left: 28px;
  color: #5a5a5a;
}

.line:hover {
  background: rgba(30, 64, 175, 0.04);
}

.gutter {
  flex: 0 0 64px;
}

.text {
  flex: 1 1 auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
