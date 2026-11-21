<!-- 首次设置向导：检测到面板未初始化时创建管理员，成功后自动登录进面板 -->
<script setup lang="ts">
import { message, UButton, UCard, UCardContent, UCardHeader, UForm, UInput, UPasswordInput } from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { reactive, shallowRef, useTemplateRef } from 'vue'
import { useRouter } from 'vue-router'

import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()

const formRef = useTemplateRef<FormExposed>('formRef')
const submitting = shallowRef(false)
const form = reactive({ setupCode: '', username: '', password: '', confirm: '' })

async function handleSubmit() {
  if (!(await formRef.value?.validate())) return
  submitting.value = true
  try {
    // setupCode 随载荷一并提交后端校验（CredentialsPayload 的扩展字段，请求层原样透传）
    const payload = { username: form.username, password: form.password, setupCode: form.setupCode.trim() }
    await auth.doSetup(payload)
    message.success('管理员创建成功，已登录')
    router.push('/dashboard')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="setup-page" @keyup.enter="handleSubmit">
    <u-card width="420">
      <u-card-header>
        <div class="setup-title">欢迎使用 proxy-ui</div>
        <div class="setup-subtitle">首次访问需输入服务器启动日志中的一次性设置码创建管理员，面板无任何默认账号口令</div>
      </u-card-header>
      <u-card-content>
        <u-form ref="formRef" :model="form" :cols="1" label-position="top">
          <u-input
            label="设置码"
            field="setupCode"
            placeholder="8 位设置码，见服务器启动日志"
            :rules="{
              required: '请输入设置码',
              minLen: [8, '设置码为 8 位'],
              maxLen: [8, '设置码为 8 位'],
            }"
          />
          <u-input
            label="用户名"
            field="username"
            placeholder="3-64 个字符"
            :rules="{
              required: '请输入用户名',
              minLen: [3, '用户名至少 3 个字符'],
              maxLen: [64, '用户名最多 64 个字符'],
            }"
          />
          <u-password-input
            label="密码"
            field="password"
            placeholder="至少 8 位"
            :rules="{ required: '请输入密码', minLen: [8, '密码至少 8 位'], maxLen: [72, '密码最多 72 位'] }"
          />
          <u-password-input
            label="确认密码"
            field="confirm"
            placeholder="再次输入密码"
            :rules="{
              required: '请再次输入密码',
              validator: (value: string) =>
                value === form.password ? '' : '两次输入的密码不一致',
            }"
          />
        </u-form>
        <u-button type="primary" class="setup-submit" :loading="submitting" @click="handleSubmit">
          创建管理员并进入面板
        </u-button>
      </u-card-content>
    </u-card>
  </div>
</template>

<style scoped>
.setup-page {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  background: var(--u-bg-color-middle);
}

.setup-title {
  font-size: var(--u-font-size-title-large);
  font-weight: 600;
}

.setup-subtitle {
  margin-top: 4px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 顶部标签形态下输入框通栏，提交按钮同宽对齐 */
.setup-submit {
  width: 100%;
  margin-top: 4px;
}
</style>
