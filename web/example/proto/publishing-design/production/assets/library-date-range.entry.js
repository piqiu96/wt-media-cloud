// Official date-range adapter shared by resource filters and production dialogs.
import { createApp, h, reactive } from 'vue';
import DateRangePicker from 'tdesign-vue-next/es/date-picker/DateRangePicker.mjs';
import 'tdesign-vue-next/es/date-picker/style/css.mjs';
const mounted = new Map();
window.LibraryDateRange = {
  mount(root) {
    for (const [host, app] of mounted) if (!host.isConnected) { app.unmount(); mounted.delete(host); }
    root.querySelectorAll('[data-library-range]').forEach(field => {
      const host = field.querySelector('[data-library-range-host]');
      if (mounted.has(host)) return;
      const inputs = [...field.querySelectorAll('input[data-field]')];
      const label = field.getAttribute('aria-labelledby');
      const name = document.getElementById(label)?.textContent || '日期范围';
      const props = reactive({
        value: inputs.every(input => input.value) ? inputs.map(input => input.value) : [],
        valueType: 'YYYY-MM-DD', format: 'YYYY-MM-DD', clearable: true,
        allowInput: false, needConfirm: false, separator: '至', placeholder: ['开始日期', '结束日期'],
        popupProps: { overlayClassName: 'lib-date-popup', attach: () => field.closest('dialog') || document.body },
        onChange(value) {
          props.value = value || [];
          inputs.forEach((input, index) => { input.value = value?.[index] || ''; input.dispatchEvent(new Event('change', { bubbles: true })); });
        }
      });
      const app = createApp({ render: () => h(DateRangePicker, props) });
      app.mount(host); mounted.set(host, app);
      host.setAttribute('role', 'group'); host.setAttribute('aria-labelledby', label);
      host.querySelectorAll('input').forEach((input, index) => {
        input.setAttribute('aria-label', name + (index ? '结束日期' : '开始日期'));
      });
    });
  }
};
