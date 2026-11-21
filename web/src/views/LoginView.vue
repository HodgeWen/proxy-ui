<!-- 登录页：账号密码登录，成功后跳回来源页或仪表盘 -->
<script setup lang="ts">
import { UButton, UCard, UCardContent, UCardHeader, UForm, UInput, UPasswordInput } from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { reactive, shallowRef, useTemplateRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const formRef = useTemplateRef<FormExposed>('formRef')
const submitting = shallowRef(false)
const form = reactive({ username: '', password: '' })

async function handleSubmit() {
  if (!(await formRef.value?.validate())) return
  submitting.value = true
  try {
    await auth.doLogin({ ...form })
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/dashboard'
    router.push(redirect)
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-page" @keyup.enter="handleSubmit">
    <u-card width="380">
      <u-card-header>
        <div class="login-title">proxy-ui</div>
        <div class="login-subtitle">登录代理面板</div>
      </u-card-header>
      <u-card-content>
        <u-form ref="formRef" :model="form" :cols="1" label-position="top">
          <u-input
            label="用户名"
            field="username"
            placeholder="请输入用户名"
            :rules="{ required: '请输入用户名' }"
          />
          <u-password-input
            label="密码"
            field="password"
            placeholder="请输入密码"
            :rules="{ required: '请输入密码' }"
          />
        </u-form>
        <u-button type="primary" class="login-submit" :loading="submitting" @click="handleSubmit">
          登录
        </u-button>
      </u-card-content>
    </u-card>
  </div>
</template>

<style scoped>
.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  background: var(--u-bg-color-middle);
}

.login-title {
  font-size: var(--u-font-size-title-large);
  font-weight: 600;
}

.login-subtitle {
  margin-top: 4px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 顶部标签形态下输入框通栏，提交按钮同宽对齐 */
.login-submit {
  width: 100%;
  margin-top: 4px;
}
</style>
