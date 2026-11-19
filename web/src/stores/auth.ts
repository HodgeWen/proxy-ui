// 鉴权状态：初始化标记、当前管理员、登录 / 登出 / 首次设置 / 改密。
import { defineStore } from 'pinia'
import { ref } from 'vue'

import {
  changePassword,
  fetchHealth,
  fetchMe,
  login,
  logout,
  setupAdmin,
} from '../api/auth'
import type { AdminInfo, CredentialsPayload } from '../api/auth'

export const useAuthStore = defineStore('auth', () => {
  /** 后端是否已创建管理员 */
  const initialized = ref(false)
  /** 当前登录管理员；null 表示未登录 */
  const user = ref<AdminInfo | null>(null)
  /** 面板版本号 */
  const version = ref('')
  /** 健康状态是否已加载（每个应用生命周期内只查一次） */
  const healthLoaded = ref(false)
  /** 页面刷新后的会话是否已校验过 */
  const sessionChecked = ref(false)

  /** 加载健康状态；失败时保持原值并向上抛出 */
  async function ensureHealth(): Promise<void> {
    if (healthLoaded.value) return
    const info = await fetchHealth()
    initialized.value = info.initialized
    version.value = info.version
    healthLoaded.value = true
  }

  /** 校验现有会话是否仍有效（每个应用生命周期内只查一次） */
  async function ensureSession(): Promise<void> {
    if (sessionChecked.value) return
    sessionChecked.value = true
    try {
      user.value = await fetchMe()
    } catch {
      user.value = null
    }
  }

  /** 登录并记录当前管理员 */
  async function doLogin(payload: CredentialsPayload): Promise<void> {
    user.value = await login(payload)
  }

  /** 首次设置：创建管理员并自动登录 */
  async function doSetup(payload: CredentialsPayload): Promise<void> {
    user.value = await setupAdmin(payload)
    initialized.value = true
    healthLoaded.value = true
  }

  /** 登出并清空本地状态 */
  async function doLogout(): Promise<void> {
    try {
      await logout()
    } finally {
      user.value = null
      sessionChecked.value = false
    }
  }

  /** 修改管理员密码 */
  async function doChangePassword(oldPassword: string, newPassword: string): Promise<void> {
    await changePassword({ oldPassword, newPassword })
  }

  return {
    initialized,
    user,
    version,
    ensureHealth,
    ensureSession,
    doLogin,
    doSetup,
    doLogout,
    doChangePassword,
  }
})
