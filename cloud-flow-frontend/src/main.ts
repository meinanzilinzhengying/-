import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import { createPinia } from 'pinia'
import { ElMessage } from 'element-plus'
import './assets/styles/common.css'

const app = createApp(App)
const pinia = createPinia()

// 错误消息去重：防止同一个错误在短时间内重复弹出提示
const errorMessages = new Map<string, number>();
const ERROR_DEDUP_INTERVAL = 3000; // 3 秒内相同消息不重复提示

function showErrorOnce(message: string, duration = 5000) {
  const key = message;
  const now = Date.now();
  const lastTime = errorMessages.get(key);
  if (lastTime && (now - lastTime) < ERROR_DEDUP_INTERVAL) {
    return; // 去重：跳过重复消息
  }
  errorMessages.set(key, now);
  // 定期清理过期的去重记录，防止内存泄漏
  if (errorMessages.size > 100) {
    const cutoff = now - ERROR_DEDUP_INTERVAL;
    for (const [k, t] of errorMessages) {
      if (t < cutoff) errorMessages.delete(k);
    }
  }
  ElMessage.error({ message, duration });
}

// Vue 全局错误处理器
app.config.errorHandler = (err, _instance, info) => {
  console.error('Vue 全局错误:', err)
  console.error('错误信息:', info)
  
  // 显示错误提示（带去重）
  showErrorOnce(`系统错误: ${(err as Error).message || '未知错误'}`)
  
  // 可以在这里添加错误上报逻辑
  // reportError(err, info)
}

// 未捕获的 Promise 异常处理
window.addEventListener('unhandledrejection', (event) => {
  console.error('未处理的 Promise 拒绝:', event.reason)
  
  // 显示错误提示（带去重）
  showErrorOnce(`请求错误: ${event.reason?.message || '网络请求失败'}`)
  
  // 阻止默认处理
  event.preventDefault()
})

// 未捕获的 JavaScript 错误处理
window.addEventListener('error', (event) => {
  console.error('未捕获的错误:', event.error)
  
  // 显示错误提示（带去重）
  showErrorOnce(`系统错误: ${event.error?.message || '未知错误'}`)
  
  // 可以在这里添加错误上报逻辑
  // reportError(event.error)
})

app.use(router)
app.use(pinia)
app.mount('#app')
