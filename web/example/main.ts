import { createApp } from 'vue'
import TDesign from 'tdesign-vue-next'
import 'tdesign-vue-next/es/style/index.css'
import '../src/styles/design-token.css'
import '../src/shared/styles/resource-module.css'
import App from './App.vue'
createApp(App).use(TDesign).mount('#app')
