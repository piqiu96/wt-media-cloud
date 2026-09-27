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
  FILE_CHOICES,
  choiceLabel,
  chosenNames,
  createLocalSettingsPage,
  describeCleanup,
  describeDiagnostic,
  describeMigration,
  describeMigrationPlan,
  describePush,
  formatBytes,
  migrationRows,
} from "./local-settings-view.js"
import { loadCandidateNames } from "./migrationCandidates.js"
import ResourceStatusBadge from "../../../../shared/ui/resource/ResourceStatusBadge.vue"

const service = createLocalSettingsService({ invoke })

const loading = ref(true)
const busy = ref("")
const settings = ref(null)
const usage = ref(null)
const draft = ref("")
const outcome = ref(null)
const failure = ref("")
// What the Agent did with the pushed location, when that is worth saying. Kept
// apart from `failure` on purpose: see `pushSaveDir`.
const pushNote = ref(null)

// The migration dialog. `migrationChoices` is keyed by file name and holds only
// 搬运/删除 — a name that is not in it is 保留 (see `migrationRows`).
const migrationVisible = ref(false)
const migrationLoading = ref(false)
const migrationBusy = ref("")
const migrationPlan = ref(null)
const migrationSummary = ref(null)
const migrationChoices = ref({})
const migrationError = ref("")
const migrationEmpty = ref(false)

const migrationRowsFor = computed(() =>
  migrationPlan.value ? migrationRows(migrationPlan.value, migrationChoices.value) : []
)

// The three choices, and 保留 is one of them rather than the absence of a button:
// every row starts as 保留, so it has to be clickable for a mistaken 搬运 to be
// taken back before anything runs.
const CHOICE_OPTIONS = [FILE_CHOICES.move, FILE_CHOICES.delete, FILE_CHOICES.keep]

const MIGRATION_COLUMNS = [
  { colKey: "name", title: "文件", ellipsis: true },
  { colKey: "bytesText", title: "体积", width: 100 },
  { colKey: "statusLabel", title: "状态", width: 130 },
  { colKey: "choice", title: "操作", width: 230 },
]

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

/**
 * Store the chosen directory, hand it to the Agent, then ask about the files.
 *
 * The order is the point. The new location is stored **before** anything is
 * asked about it, because both the push and the scan are about the directory that
 * is now chosen; and the dialog comes last so that closing it cannot leave the
 * page holding a location the file does not have.
 */
async function saveSaveDir() {
  const written = await run("save", () => service.setSaveDir(draft.value.trim()))
  if (!written) {
    return
  }
  settings.value = written
  seedDraft()
  outcome.value = { theme: "success", text: "保存位置已更新。", detail: "" }
  await pushSaveDir()
  await reload()
  await openMigration()
}

/**
 * Tell the Agent where to write from now on.
 *
 * Storing the choice writes this app's own settings file; the Agent only learns
 * the directory from this command. It is **not** fatal when it fails — the Agent
 * is not always running, and a start reads the settings file anyway — so the
 * failure gets a note rather than the page's error banner, and the button stays
 * usable. Reporting it as a failed save would be wrong twice over: the choice did
 * not fail, and it is not lost.
 */
async function pushSaveDir() {
  busy.value = "push"
  try {
    const facts = await service.pushSaveDir()
    pushNote.value = describePush(facts)
  } catch (error) {
    pushNote.value = {
      theme: "warning",
      text:
        `本机执行服务没有收到这个位置（${error?.message || String(error)}）。` +
        "它下次启动时仍会读到这个位置，已经存在那里的设置不受影响。",
    }
  } finally {
    busy.value = ""
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

/**
 * Ask what moving the previously downloaded files would involve, and show it.
 *
 * Two failure modes are kept apart here, and the dialog says which one happened:
 * a **failed read** (the history could not be listed, the scan could not run) and
 * an **empty answer** (this machine has downloaded nothing). Showing the first as
 * the second is how a person concludes their files are gone — so the catch below
 * renders the reason, and the empty case gets its own sentence.
 */
async function openMigration() {
  migrationVisible.value = true
  migrationLoading.value = true
  migrationError.value = ""
  migrationSummary.value = null
  migrationChoices.value = {}

  try {
    const names = await loadCandidateNames()
    if (!names.length) {
      migrationError.value = ""
      migrationEmpty.value = true
      return
    }
    migrationEmpty.value = false
    const plan = await service.migrationPlan(names)
    migrationPlan.value = plan
    migrationSummary.value = describeMigrationPlan(plan)
    migrationChoices.value = Object.fromEntries(plan.movable.map((file) => [file.name, FILE_CHOICES.keep]))
  } catch (error) {
    migrationPlan.value = null
    migrationError.value = error?.message || String(error) || "读取待搬运的文件失败"
  } finally {
    migrationLoading.value = false
  }
}

/** One file's decision. Only the 待搬运 rows have one. */
function choose(name, choice) {
  migrationChoices.value = { ...migrationChoices.value, [name]: choice }
}

const movableNames = computed(() => chosenNames(migrationRowsFor.value, FILE_CHOICES.move))
const deletableNames = computed(() => chosenNames(migrationRowsFor.value, FILE_CHOICES.delete))

/**
 * Carry out one of the two actions on exactly the files that were marked for it.
 *
 * Whatever the answer says is rendered by `describeMigration` — including the
 * files that were kept because the target already had that name. The dialog is
 * closed first so the report is not hidden behind it.
 */
async function runMigration(action) {
  const names = action === FILE_CHOICES.delete ? deletableNames.value : movableNames.value
  if (!names.length) {
    return
  }
  migrationBusy.value = action
  migrationError.value = ""
  try {
    const report = action === FILE_CHOICES.delete
      ? await service.deleteSavedFiles(names)
      : await service.moveSavedFiles(names)
    outcome.value = describeMigration(report, action)
    migrationVisible.value = false
    await reload()
  } catch (error) {
    migrationError.value = error?.message || String(error) || "操作失败"
  } finally {
    migrationBusy.value = ""
  }
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
          <!--
            The dialog also opens by itself right after a save; this is for coming
            back to it later — a file that was skipped, or a location that was
            changed on another day.
          -->
          <t-button
            theme="default"
            :disabled="!!busy"
            @click="openMigration"
          >
            搬运已下载的文件
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
        <!--
          改位置不会自动搬文件：新位置只影响**之后**的下载。已经下好的文件留在原来的
          位置（「打开文件」仍然找得到），要不要搬、要不要删由上面那个按钮里的弹窗决定。
        -->
        <p class="hint">
          换位置不会自动搬运已经下载的文件。它们仍在原来的位置，需要时用「搬运已下载的文件」逐个处理。
        </p>
        <t-alert
          v-if="pushNote"
          :message="pushNote.text"
          :theme="pushNote.theme"
          class="mt"
          closable
          @close="pushNote = null"
        />
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

    <!--
      搬运弹窗：先列清楚有哪些文件、各在哪儿、各多大，再由人逐个决定。

      这是**唯一**会动用户文件的地方，所以三件事都摆在明面上：哪些文件（名字与体积）、
      搬到哪儿（目标位置与所需空间）、以及哪些位置没查成（读不了的目录会让「没找到」
      不再是一个事实）。默认全部「保留」，不点就不动。
    -->
    <t-dialog
      v-model:visible="migrationVisible"
      header="搬运已下载的文件"
      width="760px"
      :close-on-overlay-click="false"
    >
      <t-loading :loading="migrationLoading" :show-overlay="true">
        <t-alert v-if="migrationError" theme="error" :message="migrationError" />
        <p v-else-if="migrationEmpty" class="hint">
          这台机器还没有下载过文件，没有可搬运的。换位置后新发起的下载会直接落到新位置。
        </p>
        <template v-else-if="migrationSummary">
          <t-alert :theme="migrationSummary.theme" :message="migrationSummary.text">
            <template #description>
              <pre class="detail">{{ migrationSummary.detail }}</pre>
            </template>
          </t-alert>
          <t-table
            v-if="migrationRowsFor.length"
            :data="migrationRowsFor"
            :columns="MIGRATION_COLUMNS"
            row-key="name"
            size="small"
            max-height="360"
            class="mt"
          >
            <template #name="{ row }">
              <div class="cell-name">{{ row.name }}</div>
              <div v-if="row.directory" class="cell-where">{{ row.directory }}</div>
            </template>
            <template #bytesText="{ row }">{{ row.bytesText }}</template>
            <template #statusLabel="{ row }">
              <ResourceStatusBadge :tone="row.statusTone" :label="row.statusLabel" />
            </template>
            <template #choice="{ row }">
              <t-space v-if="row.selectable" size="small">
                <t-button
                  v-for="option in CHOICE_OPTIONS"
                  :key="option"
                  size="small"
                  :theme="row.choice === option ? 'primary' : 'default'"
                  :variant="row.choice === option ? 'base' : 'outline'"
                  @click="choose(row.name, option)"
                >
                  {{ choiceLabel(option) }}
                </t-button>
              </t-space>
            </template>
          </t-table>
        </template>
      </t-loading>
      <template #footer>
        <t-button theme="default" :disabled="!!migrationBusy" @click="migrationVisible = false">
          关闭
        </t-button>
        <!--
          删除要单独一个按钮，而不是并进「搬运」里：两个动作对文件做的事不一样，一个
          可以后悔（搬回去），一个不可以。按钮上带着数量，按之前就能看出要动几个。
        -->
        <t-button
          theme="danger"
          variant="outline"
          :loading="migrationBusy === FILE_CHOICES.delete"
          :disabled="!!migrationBusy || !deletableNames.length"
          @click="runMigration(FILE_CHOICES.delete)"
        >
          删除选中的 {{ deletableNames.length }} 个
        </t-button>
        <t-button
          theme="primary"
          :loading="migrationBusy === FILE_CHOICES.move"
          :disabled="!!migrationBusy || !movableNames.length"
          @click="runMigration(FILE_CHOICES.move)"
        >
          搬运选中的 {{ movableNames.length }} 个
        </t-button>
      </template>
    </t-dialog>
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

/* The directory goes under the name rather than in a column of its own: it is a
   fact about that file, not a second thing to scan for. */
.cell-name {
  color: #1a1a1a;
}

.cell-where {
  color: #8a8a8a;
  font-size: 12px;
  word-break: break-all;
}
</style>
