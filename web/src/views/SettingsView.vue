<!-- 设置页骨架：账号分区提供管理员密码修改；内核 / 面板更新 / AI 供应商分区占位 -->
<script setup lang="ts">
import { message, UButton, UCard, UCardContent, UCardHeader, UEmpty, UForm, UPasswordInput, UTabs } from '@veltra/desktop'
import type { FormExposed, TabItem } from '@veltra/desktop'
import { reactive, shallowRef } from 'vue'

import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()

const active = shallowRef('account')
const tabs: TabItem[] = [
  { key: 'account', name: '账号' },
  { key: 'kernel', name: '内核' },
  { key: 'update', name: '面板更新' },
  { key: 'ai', name: 'AI 供应商' },
]

const formRef = shallowRef<FormExposed>()
const saving = shallowRef(false)
const form = reactive({ oldPassword: '', newPassword: '', confirm: '' })

async function handleSave() {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    await auth.doChangePassword(form.oldPassword, form.newPassword)
    message.success('密码已修改')
    formRef.value?.reset()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <u-tabs v-model="active" :items="tabs">
    <template #account>
      <u-card>
        <u-card-header>管理员账号</u-card-header>
        <u-card-content>
          <u-form ref="formRef" :model="form" :cols="1" label-width="88px" class="settings-form">
            <u-password-input
              label="原密码"
              field="oldPassword"
              placeholder="请输入原密码"
              :rules="{ required: '请输入原密码' }"
            />
            <u-password-input
              label="新密码"
              field="newPassword"
              placeholder="至少 8 位"
              :rules="{ required: '请输入新密码', minLen: [8, '密码至少 8 位'], maxLen: [72, '密码最多 72 位'] }"
            />
            <u-password-input
              label="确认新密码"
              field="confirm"
              placeholder="再次输入新密码"
              :rules="{
                required: '请再次输入新密码',
                validator: (value: string) =>
                  value === form.newPassword ? '' : '两次输入的密码不一致',
              }"
            />
          </u-form>
          <u-button type="primary" :loading="saving" @click="handleSave">保存密码</u-button>
        </u-card-content>
      </u-card>
    </template>

    <template #kernel>
      <u-card>
        <u-card-header>sing-box 内核</u-card-header>
        <u-card-content>
          <u-empty text="内核版本下载、启停与日志管理将在后续版本提供" block />
        </u-card-content>
      </u-card>
    </template>

    <template #update>
      <u-card>
        <u-card-header>面板更新</u-card-header>
        <u-card-content>
          <u-empty text="面板在线检查更新与一键升级将在后续版本提供" block />
        </u-card-content>
      </u-card>
    </template>

    <template #ai>
      <u-card>
        <u-card-header>AI 供应商</u-card-header>
        <u-card-content>
          <u-empty text="OpenAI 兼容供应商配置将在后续版本提供" block />
        </u-card-content>
      </u-card>
    </template>
  </u-tabs>
</template>

<style scoped>
.settings-form {
  max-width: 420px;
  margin-bottom: 16px;
}
</style>
