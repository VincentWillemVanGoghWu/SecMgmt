import { createApp } from 'vue'

import { permissionDirective } from './directives/permission'
import { refreshOnEmptyDirective } from './directives/refreshOnEmpty'
import App from './App.vue'
import { router } from './router'
import { installOperationLogTracker } from './services/operationLogTracker'
import { pinia } from './stores'
import 'element-plus/dist/index.css'
import './style.css'

const app = createApp(App)

app.use(pinia)
app.use(router)
app.directive('permission', permissionDirective)
app.directive('refresh-on-empty', refreshOnEmptyDirective)
installOperationLogTracker(router)

app.mount('#app')
