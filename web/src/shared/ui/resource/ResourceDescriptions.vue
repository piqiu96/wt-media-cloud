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
      return h('div', { ref: container, class: 'resource-descriptions', 'data-columns': layout.columns }, [
        h(Descriptions, { ...attrs, column: layout.columns, itemLayout: 'horizontal', tableLayout: 'fixed', colon: false }, {
          default: () => fields.map((node, index) => cloneVNode(node, { span: layout.spans[index] })),
        }),
      ])
    }
  },
})
</script>

<style scoped>
.resource-descriptions { min-width: 0; }
.resource-descriptions :deep(.t-descriptions__body) { width: 100%; }
.resource-descriptions :deep(.t-descriptions__label) { width: 112px; padding: 8px 16px 8px 0; color: var(--wt-text-tertiary); font-size: 14px; font-weight: 400; line-height: 24px; vertical-align: top; white-space: normal; }
.resource-descriptions :deep(.t-descriptions__content) { padding: 8px 16px 8px 0; color: var(--wt-text-primary); font-size: 14px; line-height: 24px; vertical-align: top; overflow-wrap: anywhere; white-space: pre-wrap; }
.resource-descriptions :deep(.t-descriptions__content .t-button) { margin-left: 8px; }
@media (max-width: 600px) { .resource-descriptions :deep(.t-descriptions__label) { width: 96px; } }
</style>
