import { onMounted } from 'vue'
import { auth } from './api'

// useRouterGuard：挂载时确保登录态已就绪（供需要 auth 上下文的页面调用）。
export function useRouterGuard() {
  onMounted(async () => { await auth.me() })
}
