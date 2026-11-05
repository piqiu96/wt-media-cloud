import { shallowRef } from 'vue'
export function createOverlayStack() {
 const items=shallowRef([])
 return {
  items,
  open(id){if(!items.value.includes(id))items.value=[...items.value,id]},
  close(id){items.value=items.value.filter(item=>item!==id)},
  isTop(id){return items.value.at(-1)===id},
 }
}
export const overlayStack=createOverlayStack()
