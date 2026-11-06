export function overlayFocusables(root) {
  return [...(root?.querySelectorAll('button,a[href],input,select,textarea,[tabindex]') || [])]
    .filter(node => !node.disabled && node.tabIndex >= 0 && !node.closest('[inert]') && node.getClientRects().length > 0)
}
export function nextOverlayFocus(root, active, backwards) {
  const nodes = overlayFocusables(root)
  if (!nodes.length) return root
  const index = nodes.indexOf(active)
  if (index < 0) return backwards ? nodes.at(-1) : nodes[0]
  return nodes[(index + (backwards ? -1 : 1) + nodes.length) % nodes.length]
}
