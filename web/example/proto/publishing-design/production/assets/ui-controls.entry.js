// Uses the same official Switch as the application; no business-specific switch drawing.
import { createApp, h, reactive } from 'vue';
import { Switch } from 'tdesign-vue-next/es/switch/index.mjs';
import 'tdesign-vue-next/es/style/index.css';
import 'tdesign-vue-next/es/switch/style/css.mjs';
const mounted = new Map();
window.OPSControls = {
  mount(root, getProps) {
    for (const [element, item] of mounted) if (!element.isConnected) { item.app.unmount(); mounted.delete(element); }
    root.querySelectorAll('[data-strategy-switch]').forEach(element => {
      const props = reactive(getProps(Number(element.dataset.strategySwitch)));
      const app = createApp({ render: () => h(Switch, props) });
      app.mount(element); mounted.set(element, { app, props });
    });
  },
  update(id, values) { for (const [element, item] of mounted) if (Number(element.dataset.strategySwitch) === id) Object.assign(item.props, values); }
};
