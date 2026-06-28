import { createApp } from 'vue'

import { permissionDirective } from './directives/permission'
import App from './App.vue'
import { router } from './router'
import { pinia } from './stores'
import 'element-plus/dist/index.css'
import './style.css'

const app = createApp(App)

app.use(pinia)
app.use(router)
app.directive('permission', permissionDirective)

app.mount('#app')
