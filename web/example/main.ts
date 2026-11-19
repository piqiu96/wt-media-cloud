import { createApp } from 'vue'
import TDesign from 'tdesign-vue-next'
import 'tdesign-vue-next/es/style/index.css'
import '../src/styles/design-token.css'
import '../src/shared/styles/resource-module.css'
import './common/number-stepper.css'
import '../src/shared/styles/resource-form-actions.css'
import App from './App.vue'
document.body.classList.add('resource-form-actions-scope')
createApp(App).use(TDesign).mount('#app')
