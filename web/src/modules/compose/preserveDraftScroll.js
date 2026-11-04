import { nextTick } from 'vue'

// Structural form edits keep the current viewport, even when browser anchoring
// would otherwise follow the moved card. Values remain on their keyed objects.
export function preserveDraftScroll(element, change) {
  const containers = []
  for (let node = element; node; node = node.parentElement) {
    if (node.scrollHeight > node.clientHeight) containers.push([node, node.scrollTop, node.scrollLeft])
  }
  change()
  nextTick(() => {
    for (const [node, top, left] of containers) {
      node.scrollTop = top
      node.scrollLeft = left
    }
  })
}
