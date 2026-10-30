import { createRenderer, defineComponent, h, nextTick, ref } from 'vue'
import { describe, expect, it } from 'vitest'
import * as Vue from 'vue'
import { readFileSync } from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { transformSync } from 'esbuild'
import { CATEGORY_TYPE_OPTIONS } from '../../utils/categories.js'
const control = name => defineComponent({ inheritAttrs: false, props: { filterable: Boolean, clearable: Boolean, disabled: Boolean, loading: Boolean }, setup(props, { attrs, slots }) { return () => h(name, { ...attrs, ...props }, slots.default?.()) } })
const controls = { Select: control('select-control'), Option: control('option-control'), CheckboxGroup: control('checkbox-group'), Checkbox: control('checkbox-control'), Input: control('input-control'), Tag: control('tag-control') }
// Vitest's node mode compiles SFCs for SSR. Compile these three actual templates
// for a client renderer so the tests can exercise their event/model bindings.
function clientComponent(file, dependencies = {}) {
  const { descriptor } = parse(readFileSync(new URL(file, import.meta.url), 'utf8'))
  const script = compileScript(descriptor, { id: file, inlineTemplate: true })
  const code = transformSync(script.content, { format: 'cjs', loader: 'js' }).code
  const module = { exports: {} }
  new Function('require', 'module', 'exports', code)(id => ({ vue: Vue, 'tdesign-vue-next': controls, '../../utils/categories.js': { CATEGORY_TYPE_OPTIONS }, ...dependencies })[id], module, module.exports)
  return module.exports.default
}
const BusinessSelect = clientComponent('./BusinessSelect.vue')
const CategoryFields = clientComponent('./CategoryFields.vue', { './BusinessSelect.vue': BusinessSelect })
const TagInput = clientComponent('./TagInput.vue')

// Minimal Vue host: exercises live v-model/events without a browser or simulated network.
const renderer = createRenderer({
  createElement: type => ({ type, children: [], props: {} }),
  createText: text => ({ text }), createComment: text => ({ text }),
  setText: (node, text) => { node.text = text }, setElementText: (node, text) => { node.text = text },
  patchProp: (node, key, _, value) => { node.props[key] = value },
  insert(node, parent, anchor) {
    if (node.parent) node.parent.children = node.parent.children.filter(n => n !== node)
    node.parent = parent
    const index = anchor ? parent.children.indexOf(anchor) : -1
    parent.children.splice(index < 0 ? parent.children.length : index, 0, node)
  },
  remove(node) { if (node?.parent) node.parent.children = node.parent.children.filter(n => n !== node) },
  parentNode: node => node.parent, nextSibling: node => node.parent?.children[node.parent.children.indexOf(node) + 1] || null,
})
function mount(render) { const root = { children: [] }; const app = renderer.createApp(defineComponent({ setup: () => render })); app.mount(root); return { root, app } }
function findAll(root, type) { return (root.type === type ? [root] : []).concat((root.children || []).flatMap(n => findAll(n, type))) }
const catalog = [{ id: 'g', type: 'game', name: '三角洲' }, { id: 'a', type: 'ai', name: 'AI工具' }]

describe('common business fields', () => {
  it('preserves values, disabled/loading and explicit clear policy while providing searchable team/member inputs', () => {
    const value = ref(12)
    const { root, app } = mount(() => h(BusinessSelect, { kind: 'member', filter: true, modelValue: value.value, 'onUpdate:modelValue': v => { value.value = v }, disabled: true, loading: true, clearable: false, options: [{ value: 12, label: '运营甲' }] }))
    const select = findAll(root, 'select-control')[0]
    expect(select.props).toMatchObject({ filterable: true, clearable: false, disabled: true, loading: true, placeholder: '全部', modelValue: 12 })
    select.props['onUpdate:modelValue'](13)
    expect(value.value).toBe(13)
    app.unmount()
  })
  it('clears second-level choices when switching or clearing the first level; never carries a hidden category', async () => {
    const type = ref('game'), ids = ref(['g'])
    const { root, app } = mount(() => h(CategoryFields, { categories: catalog, type: type.value, 'onUpdate:type': v => { type.value = v }, modelValue: ids.value, 'onUpdate:modelValue': v => { ids.value = v } }))
    findAll(root, 'select-control')[0].props.onChange('ai')
    await nextTick()
    expect(type.value).toBe('ai'); expect(ids.value).toEqual([])
    expect(findAll(root, 'checkbox-control').map(n => n.props.value)).toEqual(['a'])
    findAll(root, 'checkbox-group')[0].props['onUpdate:modelValue'](['a'])
    expect(ids.value).toEqual(['a'])
    findAll(root, 'select-control')[0].props.onChange('')
    await nextTick()
    expect(ids.value).toEqual([]); expect(findAll(root, 'checkbox-control')).toHaveLength(0)
    app.unmount()
  })
  it('keeps second-level filter disabled until a parent is chosen', () => {
    const { root, app } = mount(() => h(CategoryFields, { filter: true, categories: catalog }))
    expect(findAll(root, 'select-control')[1].props.disabled).toBe(true)
    app.unmount()
  })
  it('adds trimmed tags with Enter, deduplicates them, and removes the selected tag', async () => {
    const tags = ref(['矩阵'])
    const { root, app } = mount(() => h(TagInput, { modelValue: tags.value, 'onUpdate:modelValue': v => { tags.value = v } }))
    const input = () => findAll(root, 'input-control')[0]
    input().props['onUpdate:modelValue']('  日更  '); await nextTick(); input().props.onEnter(); await nextTick()
    expect(tags.value).toEqual(['矩阵', '日更'])
    input().props['onUpdate:modelValue']('日更'); await nextTick(); input().props.onEnter(); await nextTick()
    expect(tags.value).toEqual(['矩阵', '日更'])
    findAll(root, 'tag-control')[0].props.onClose(); await nextTick()
    expect(tags.value).toEqual(['日更'])
    app.unmount()
  })
})
