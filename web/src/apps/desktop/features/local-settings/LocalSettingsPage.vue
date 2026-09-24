<script setup>
import { computed, onMounted, ref } from "vue"
// Static import, as in `AccountsPage.vue` and `LocalLogsPage.vue`: a dynamic
// import() of a bare specifier does not resolve in the packaged Tauri WebView
// ("Module name ... does not resolve to a valid file"). It is inert in Cloud
// Web, which never reaches the code that calls it.
import { invoke } from "@tauri-apps/api/core"
import {
  createLocalSettingsService,
  DEFAULT_TAIL_LINES,
} from "./service.js"
import {
  createLocalSettingsPage,
  describeCleanup,
  describeDiagnostic,
  formatBytes,
} from "./local-settings-view.js"

const service = createLocalSettingsService({ invoke })

const loading = ref(true)
const busy = ref("")
const settings = ref(null)
const usage = ref(null)
const draft = ref("")
const outcome = ref(null)
const failure = ref("")

// The two log trees the page offers a cleanup for. Each is one button, and the
// label is the tree's own, so the button and the report that comes back name the
// same thing.
const LOG_TREES = [
  { source: "desktop", label: "本机控制台日志" },
  { source: "agent", label: "本机执行服务日志" },
]

const page = computed(() => createLocalSettingsPage({ settings: settings.value, usage: usage.value }))

// The typed path is only a draft until it is stored, so the input is seeded from
// what the file says and is not written back on every keystroke: a person
// half-way through typing a path has not chosen it.
function seedDraft() {
  draft.value = settings.value?.saveDir ?? ""
}

async function run(key, work) {
  busy.value = key
  outcome.value = null
  failure.value = ""
  try {
    return await work()
  } catch (error) {
    // The Rust side's messages already name the path and the reason; this only
    // adds a fallback for a non-Error throw.
    failure.value = error?.message || String(error) || "操作失败"
    return null
  } finally {
    busy.value = ""
  }
}

async function reload() {
  loading.value = true
  try {
    const [nextSettings, nextUsage] = await Promise.all([
      run("settings", () => service.getSettings()),
      run("usage", () => service.storageUsage()),
    ])
    if (nextSettings) {
      settings.value = nextSettings
      seedDraft()
    }
    if (nextUsage) {
      usage.value = nextUsage
    }
  } finally {
    loading.value = false
  }
}

async function saveSaveDir() {
  const written = await run("save", () => service.setSaveDir(draft.value.trim()))
  if (written) {
    settings.value = written
    seedDraft()
    outcome.value = { theme: "success", text: "保存位置已更新。", detail: "" }
    await reload()
  }
}

// Clearing is a value the file can hold, not a deletion of the file — so it goes
// through the same command with `null` rather than by emptying the input.
async function clearSaveDir() {
  const written = await run("clear", () => service.setSaveDir(null))
  if (written) {
    settings.value = written
    seedDraft()
    outcome.value = { theme: "success", text: "已清除保存位置。", detail: "" }
    await reload()
  }
}

async function openPlace(place) {
  await run(`open-${place}`, () => service.openPlace(place))
}

async function cleanCache() {
  const report = await run("cache", () => service.cacheCleanup())
  if (report) {
    outcome.value = describeCleanup(report)
    await reload()
  }
}

async function cleanLogs(source) {
  const report = await run(`logs-${source}`, () => service.logCleanup(source))
  if (report) {
    outcome.value = describeCleanup(report)
    await reload()
  }
}

async function exportDiagnostic() {
  const report = await run("export", () => service.diagnosticExport())
  if (report) {
    outcome.value = describeDiagnostic(report)
  }
}

onMounted(reload)
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <div class="settings-page">
      <t-card title="保存位置" :bordered="true">
        <template #description>
          素材下载与成片的保存位置。留空表示未设置，由任务自行决定。
        </template>
        <div class="field">
          <t-input
            v-model="draft"
            placeholder="/Users/你的用户名/Movies/WTMedia"
            :disabled="!!busy"
          />
        </div>
        <div class="actions">
          <t-button theme="primary" :loading="busy === 'save'" :disabled="!!busy" @click="saveSaveDir">
            保存
          </t-button>
          <t-button
            theme="default"
            :loading="busy === 'clear'"
            :disabled="!!busy || !page.saveDirSet"
            @click="clearSaveDir"
          >
            清除
          </t-button>
          <t-button
            theme="default"
            :loading="busy === 'open-data'"
            :disabled="!!busy"
            @click="openPlace('data')"
          >
            打开设置文件夹
          </t-button>
        </div>
        <t-descriptions :column="1" bordered size="small" class="facts">
          <t-descriptions-item label="当前保存位置">
            {{ page.saveDirText }}
          </t-descriptions-item>
          <t-descriptions-item v-if="page.fileSet" label="设置文件">
            {{ page.fileText }}
          </t-descriptions-item>
        </t-descriptions>
        <p class="hint">
          这个位置必须是已经存在的绝对路径。本页无法替你新建目录，也不会替你新建：填一个不存在的路径会被拒绝，因为写不进去的任务比一句提示更难查。
        </p>
      </t-card>

      <t-card title="存储与日志" :bordered="true" class="mt">
        <template #description>
          磁盘可用空间、缓存占用，以及两侧日志各占多少。
        </template>
        <t-descriptions v-if="usage" :column="1" bordered size="small">
          <t-descriptions-item label="磁盘可用空间">
            {{ page.availableText }}
          </t-descriptions-item>
          <t-descriptions-item label="缓存占用">
            {{ page.cacheText }}
          </t-descriptions-item>
        </t-descriptions>
        <t-list v-if="page.logTrees.length" size="small" class="trees">
          <t-list-item v-for="tree in page.logTrees" :key="tree.source">
            <t-list-item-meta
              :title="`${tree.source} 日志`"
              :description="tree.directory"
            />
            <template #action>
              <span class="tree-fact">{{ tree.isEmpty ? "暂无文件" : tree.bytesText }}</span>
              <span class="tree-fact muted">{{ tree.filesText }}</span>
              <t-button
                theme="default"
                size="small"
                :disabled="!!busy"
                @click="openPlace(tree.source)"
              >
                打开文件夹
              </t-button>
            </template>
          </t-list-item>
        </t-list>
        <t-empty v-else description="未读到日志目录" />
      </t-card>

      <t-card title="清理" :bordered="true" class="mt">
        <template #description>
          只清理可以安全再生或已轮转的文件；正在写入的日志、素材、成片不会被删除。
        </template>
        <div class="actions">
          <t-button theme="default" :loading="busy === 'cache'" :disabled="!!busy" @click="cleanCache">
            清理缓存
          </t-button>
          <t-button
            v-for="tree in LOG_TREES"
            :key="tree.source"
            theme="default"
            :loading="busy === `logs-${tree.source}`"
            :disabled="!!busy"
            @click="cleanLogs(tree.source)"
          >
            清理{{ tree.label }}
          </t-button>
        </div>
        <p class="hint">
          按天保留由轮转器负责（默认 14 天），这一页只是让你在需要时立刻回收；不需要控制总量。
        </p>
      </t-card>

      <t-card title="诊断导出" :bordered="true" class="mt">
        <template #description>
          版本、组件状态、脱敏日志与失败任务摘要，打成单个归档。日志在写入归档之前已经脱敏，导出不会重新引入凭据。
        </template>
        <div class="actions">
          <t-button theme="primary" :loading="busy === 'export'" :disabled="!!busy" @click="exportDiagnostic">
            导出脱敏诊断包
          </t-button>
          <span class="hint inline">尾部截取上限 {{ DEFAULT_TAIL_LINES }} 行/文件</span>
        </div>
      </t-card>

      <t-alert
        v-if="outcome"
        :message="outcome.text"
        :theme="outcome.theme"
        class="mt"
      >
        <template v-if="outcome.detail" #description>
          <pre class="detail">{{ outcome.detail }}</pre>
        </template>
      </t-alert>
      <t-alert v-if="failure" :message="failure" theme="error" class="mt" />
    </div>
  </t-loading>
</template>

<style scoped>
.settings-page {
  margin: 16px;
}

.mt {
  margin-top: 16px;
}

.field {
  max-width: 560px;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 12px;
}

.facts {
  margin-top: 16px;
}

.trees {
  margin-top: 8px;
}

.tree-fact {
  margin-left: 12px;
  white-space: nowrap;
}

.tree-fact.muted {
  color: #8a8a8a;
}

.hint {
  margin: 12px 0 0;
  color: #8a8a8a;
  font-size: 12px;
  line-height: 1.7;
}

.hint.inline {
  margin: 0;
  align-self: center;
}

.detail {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
