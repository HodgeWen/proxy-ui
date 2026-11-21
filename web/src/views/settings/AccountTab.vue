<!-- 设置页「账号」页签：管理员修改密码（原密码校验 + 两次输入一致性） -->
<script setup lang="ts">
import { UButton, UCard, UCardContent, UCardHeader, UForm, UPasswordInput, message } from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { reactive, shallowRef, useTemplateRef } from 'vue'

import { useAuthStore } from '../../stores/auth'

const auth = useAuthStore()

const formRef = useTemplateRef<FormExposed>('formRef')
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
  <u-card>
    <u-card-header>
      <div class="card-header-row">
        <span>管理员账号</span>
        <span class="header-meta">当前账号：{{ auth.user?.username || '-' }}</span>
      </div>
    </u-card-header>
    <u-card-content>
      <u-form ref="formRef" :model="form" :cols="2" label-width="96px" class="settings-form">
        <u-password-input
          label="原密码"
          field="oldPassword"
          :span="1"
          placeholder="请输入原密码"
          tips="当前登录账号的密码，修改密码前需先验证身份"
          :rules="{ required: '请输入原密码' }"
        />
        <u-password-input
          label="新密码"
          field="newPassword"
          :span="1"
          placeholder="至少 8 位"
          tips="新密码至少 8 位、最多 72 位；建议混合字母、数字与符号"
          :rules="{ required: '请输入新密码', minLen: [8, '密码至少 8 位'], maxLen: [72, '密码最多 72 位'] }"
        />
        <u-password-input
          label="确认新密码"
          field="confirm"
          :span="1"
          placeholder="再次输入新密码"
          tips="再次输入一遍新密码，两次不一致将无法保存"
          :rules="{
            required: '请再次输入新密码',
            validator: (value: string) =>
              value === form.newPassword ? '' : '两次输入的密码不一致',
          }"
        />
      </u-form>
      <div class="card-actions">
        <u-button type="primary" :loading="saving" @click="handleSave">保存密码</u-button>
      </div>
    </u-card-content>
  </u-card>
</template>

<style scoped>
/* card-header-row / card-actions 为 App.vue 全局类 */

.settings-form {
  margin-bottom: 4px;
}

.header-meta {
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}
</style>
