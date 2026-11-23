<script>
import { defineComponent, h, ref, onMounted, onBeforeUnmount, Fragment, Comment, cloneVNode } from 'vue'
import { Descriptions } from 'tdesign-vue-next'
import { descriptionLayout } from './descriptionLayout.js'

function items(nodes) {
  return nodes.flatMap(node => node.type === Fragment ? items(node.children || []) : node.type === Comment ? [] : [node])
}
export default defineComponent({
  name: 'ResourceDescriptions',
  inheritAttrs: false,
  setup(_, { attrs, slots }) {
    const container = ref(null), width = ref(0)
    let observer
    onMounted(() => {
      observer = new ResizeObserver(([entry]) => { width.value = entry.contentRect.width })
      observer.observe(container.value)
    })
    onBeforeUnmount(() => observer?.disconnect())
    return () => {
      const fields = items(slots.default?.() || [])
      const layout = descriptionLayout(fields.map(node => node.props || {}), width.value)
      return h('div', { ref: container, class: 'resource-descriptions', 'data-columns': layout.columns, 'data-compact': width.value > 0 && width.value < 360 }, [
        h(Descriptions, { ...attrs, column: layout.columns, itemLayout: 'horizontal', tableLayout: 'fixed', colon: false }, {
          default: () => fields.map((node, index) => cloneVNode(node, { span: layout.spans[index] })),
        }),
      ])
    }
  },
})
</script>

<style scoped>
.resource-descriptions { min-width: 0; container-type: inline-size; }
.resource-descriptions :deep(.t-descriptions__body) { width: 100%; }
/* A full first row spans three cells in TDesign. Native fixed table layout
   then ignores later label widths. Explicit tracks keep both field pairs
   aligned regardless of which row contains the first long value. */
.resource-descriptions :deep(.t-descriptions__body > tbody > tr) { display: grid; grid-template-columns: 112px minmax(0, 1fr); column-gap: 16px; }
.resource-descriptions[data-columns="2"] :deep(.t-descriptions__body > tbody > tr) { grid-template-columns: 112px minmax(0, 1fr) 112px minmax(0, 1fr); }
.resource-descriptions :deep(td[colspan="3"]) { grid-column: span 3; }
.resource-descriptions :deep(.t-descriptions__body .t-descriptions__label), .resource-descriptions :deep(.t-descriptions__body .t-descriptions__content) { width: auto; min-width: 0; }
.resource-descriptions :deep(.t-descriptions__label) { width: 112px; padding: 8px 16px 8px 0; color: var(--wt-text-tertiary); font-size: 13px; font-weight: 500; line-height: 24px; vertical-align: top; white-space: normal; }
.resource-descriptions :deep(.t-descriptions__content) { padding: 8px 16px 8px 0; color: var(--wt-text-primary); font-size: 14px; line-height: 24px; vertical-align: top; overflow-wrap: anywhere; white-space: pre-wrap; }
.resource-descriptions :deep(.t-descriptions__content .t-button) { margin-left: 8px; }
@media (max-width: 600px) { .resource-descriptions :deep(.t-descriptions__body > tbody > tr) { grid-template-columns: 96px minmax(0, 1fr); column-gap: 12px; } }
.resource-descriptions[data-compact="true"] :deep(.t-descriptions__body > tbody > tr) { grid-template-columns: 96px minmax(0, 1fr); column-gap: 12px; }
.resource-descriptions[data-compact="true"] :deep(.t-descriptions__label) { padding: 8px 0; }
.resource-descriptions[data-compact="true"] :deep(.t-descriptions__content) { padding: 8px 0; }
</style>
