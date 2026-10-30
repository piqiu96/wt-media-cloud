import { readFileSync } from 'node:fs'
import { compile } from '@vue/compiler-dom'
import * as Vue from 'vue'
import { renderToString } from '@vue/server-renderer'
import { expect, it } from 'vitest'
import { platformLabel } from '../../../shared/utils/platforms.js'

it('renders the actual edit template when a hidden overlay eagerly evaluates its slots', async () => {
  const source = readFileSync(new URL('./AccountsPage.vue', import.meta.url), 'utf8')
  const template = source.match(/<ResourceOverlay :visible="editVisible"[^]*?<\/ResourceOverlay>/)[0]
  const render = new Function('Vue', compile(template, { mode: 'function', prefixIdentifiers: true }).code)(Vue)
  const app = Vue.createSSRApp({ render, setup: () => ({ editVisible: false, editAccount: null, savingEdit: false, editError: '', editCategoryIds: [], editCategoryType: '', editTags: [], editRemark: '', categories: [], platformLabel, accountDisplayName: () => '—', closeAccountForm: () => {}, saveEdit: () => {} }) })
  for (const name of new Set([...template.matchAll(/<([\w-]+)/g)].map(m => m[1]).filter(n => n !== 'template'))) {
    app.component(name, { inheritAttrs: false, setup: (_, { slots }) => () => Vue.h('div', slots.default?.()) })
  }
  await expect(renderToString(app)).resolves.toContain('—')
})
