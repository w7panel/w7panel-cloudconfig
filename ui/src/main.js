import { createApp } from 'vue'
import ArcoVue, { Message } from '@arco-design/web-vue'
import ArcoVueIcon from '@arco-design/web-vue/es/icon'
import '@arco-design/web-vue/dist/arco.css'
import './style.css'
import App from './App.vue'
import { bootstrapAuth } from './auth'

async function mount() {
  try {
    const token = await bootstrapAuth()
    if (!token && !window.__POWERED_BY_WUJIE__) return
  } catch (error) {
    Message.error(error?.response?.data?.message || error.message || '登录失败')
  }
  createApp(App).use(ArcoVue).use(ArcoVueIcon).mount('#app')
}

mount()
