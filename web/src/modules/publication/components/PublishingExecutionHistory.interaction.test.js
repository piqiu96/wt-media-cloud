import { createRenderer, h, defineComponent, nextTick } from 'vue';
import * as Vue from 'vue';
import { readFileSync } from 'node:fs';
import { parse, compileScript } from '@vue/compiler-sfc';
import { transformSync } from 'esbuild';
import { afterEach, expect, it, vi } from 'vitest';
import * as presentation from '../executionPresentation.js';
import * as dateTime from '../dateTime.js';
// Only shared visual primitives use a headless host. Both business templates
// are compiled for the client, so clicks exercise their real selection binding.
const shell = name => defineComponent({ setup: (_, { slots }) => () => h(name, {}, slots.default?.()) });
const Section = defineComponent({ props: ['title'], setup: (props, { slots }) => () => h('section', {}, [h('h3', props.title), slots.default?.()]) });
const Overlay = defineComponent({ props: ['visible'], emits: ['update:visible'], setup: (props, { slots, emit }) => () => props.visible ? h('dialog', {}, [h('button', { onClick: () => emit('update:visible', false) }, '关闭'), slots.default?.()]) : null });
const Badge = defineComponent({ props: ['label'], setup: props => () => h('span', props.label) });
const controls = { Button: defineComponent({ setup: (_, { attrs, slots }) => () => h('button', attrs, slots.default?.()) }), DescriptionsItem: shell('dd') };
const deps = {
  vue: Vue, 'tdesign-vue-next': controls, 'tdesign-icons-vue-next': { ImageIcon: shell('i') },
  '../../../shared/ui/resource/ResourceSection.vue': Section,
  '../../../shared/ui/resource/ResourceDescriptions.vue': shell('dl'),
  '../../../shared/ui/resource/ResourceStatusBadge.vue': Badge,
  '../../../shared/ui/resource/ResourceOverlay.vue': Overlay,
  '../../../shared/ui/resource/ResourceNotice.vue': shell('aside'),
  '../../../shared/ui/resource/ResourceRowActions.vue': shell('div'),
  '../executionPresentation.js': presentation, '../dateTime.js': dateTime,
};
function client(file) {
  const { descriptor } = parse(readFileSync(new URL(file, import.meta.url), 'utf8'));
  const code = transformSync(compileScript(descriptor, { id: file, inlineTemplate: true }).content, { format: 'cjs' }).code;
  const module = { exports: {} };
  new Function('require', 'module', 'exports', code)(id => deps[id], module, module.exports);
  return module.exports.default;
}
deps['./PublishingAttemptDetail.vue'] = client('./PublishingAttemptDetail.vue');
const History = client('./PublishingExecutionHistory.vue');
const portal = { children: [] };
const renderer = createRenderer({
  querySelector: () => portal,
  createElement: type => ({ type, children: [], props: {}, closest: () => null }),
  createText: text => ({ text }), createComment: text => ({ text: '' }),
  setText: (node, text) => { node.text = text; }, setElementText: (node, text) => { node.text = text; },
  patchProp: (node, key, _, value) => { node.props[key] = value; },
  insert(node, parent, anchor) { if (node.parent) node.parent.children = node.parent.children.filter(n => n !== node); node.parent = parent; const at = parent.children.indexOf(anchor); parent.children.splice(at < 0 ? parent.children.length : at, 0, node); },
  remove(node) { node.parent.children = node.parent.children.filter(n => n !== node); },
  parentNode: node => node.parent, nextSibling: node => node.parent?.children[node.parent.children.indexOf(node) + 1],
});
const nodes = (node, type) => [...(node.type === type ? [node] : []), ...(node.children || []).flatMap(child => nodes(child, type))];
const text = node => String(node.text || '') + (node.children || []).map(text).join('');
let app;
afterEach(() => { app?.unmount(); vi.unstubAllGlobals(); });
it('opens the clicked historical snapshot, closes it and opens another without changing the records', async () => {
  vi.stubGlobal('document', { getElementById: () => null });
  vi.stubGlobal('window', { addEventListener() {}, removeEventListener() {} });
  const attempts = [
    { id:'a', attempt_no:1, status:'completed', result_status:'failed', input_snapshot:{content:{description:'旧执行标题'}} },
    { id:'b', attempt_no:2, status:'running', input_snapshot:{content:{description:'新执行标题'}} },
  ];
  const root = { children: [] };
  app = renderer.createApp(History, { task:{id:'task'}, attempts }); app.mount(root); await nextTick();
  expect(nodes(root,'dialog')).toHaveLength(0);
  nodes(root,'button').find(button => button.props['aria-label'] === '查看第 1 次执行详情').props.onClick(); await nextTick();
  expect(text(nodes(root,'dialog')[0])).toContain('旧执行标题');
  expect(text(nodes(root,'dialog')[0])).not.toContain('新执行标题');
  nodes(root,'dialog')[0].children[0].props.onClick(); await nextTick();
  expect(nodes(root,'dialog')).toHaveLength(0);
  nodes(root,'button').find(button => button.props['aria-label'] === '查看第 2 次执行详情').props.onClick(); await nextTick();
  expect(text(nodes(root,'dialog')[0])).toContain('新执行标题');
  expect(attempts[0].id).toBe('a');
});
