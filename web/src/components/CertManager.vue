<!-- 证书管理：上传/粘贴 PEM 存储与服务器本地路径引用两种方式的增删改查 -->
<script setup lang="ts">
import { createCert, deleteCert, listCerts, updateCert } from '../api/cert'
import type { CertPayload, Certificate } from '../api/cert'
import {
  UButton,
  UDialog,
  UForm,
  UInput,
  USelect,
  UTable,
  UTag,
  UTextarea,
  defineTableColumns,
  message,
  messageConfirm,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { reactive, shallowRef } from 'vue'

const emit = defineEmits<{ changed: [] }>()

const visible = shallowRef(false)
const certs = shallowRef<Certificate[]>([])
const loading = shallowRef(false)

const columns = defineTableColumns([
  { key: 'name', name: '名称', minWidth: 140 },
  { key: 'mode', name: '引用方式', width: 110, align: 'center' },
  { key: 'paths', name: '证书 / 私钥', minWidth: 220 },
  { key: 'actions', name: '操作', width: 110, align: 'center' },
])

async function refresh(): Promise<void> {
  loading.value = true
  try {
    certs.value = await listCerts()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    loading.value = false
  }
}

function open(): void {
  visible.value = true
  void refresh()
}

// ---------- 新增 / 编辑 ----------

const formVisible = shallowRef(false)
const editing = shallowRef<Certificate | null>(null)
const saving = shallowRef(false)
const formRef = shallowRef<FormExposed>()
const certFileInput = shallowRef<HTMLInputElement>()
const keyFileInput = shallowRef<HTMLInputElement>()
const form = reactive({
  name: '',
  mode: 'managed' as 'managed' | 'path',
  certPem: '',
  keyPem: '',
  certPath: '',
  keyPath: '',
})

const modeOptions = [
  { label: '上传 / 粘贴存储', value: 'managed' },
  { label: '服务器本地路径', value: 'path' },
]

function openCreate(): void {
  editing.value = null
  Object.assign(form, { name: '', mode: 'managed', certPem: '', keyPem: '', certPath: '', keyPath: '' })
  formVisible.value = true
}

function openEdit(row: Certificate): void {
  editing.value = row
  Object.assign(form, { name: row.name, mode: row.mode, certPem: '', keyPem: '', certPath: row.certPath, keyPath: row.keyPath })
  formVisible.value = true
}

/** 读取本地证书/私钥文件内容填入文本域 */
async function pickPem(target: 'certPem' | 'keyPem', event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) form[target] = await file.text()
  input.value = ''
}

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    const payload: CertPayload =
      form.mode === 'managed'
        ? { name: form.name, mode: 'managed', ...(form.certPem || form.keyPem ? { certPem: form.certPem, keyPem: form.keyPem } : {}) }
        : { name: form.name, mode: 'path', ...(form.certPath || form.keyPath ? { certPath: form.certPath, keyPath: form.keyPath } : {}) }
    if (editing.value) await updateCert(editing.value.id, payload)
    else await createCert(payload)
    message.success(editing.value ? '证书已更新' : '证书已添加')
    formVisible.value = false
    await refresh()
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: Certificate): Promise<void> {
  const action = await messageConfirm({
    title: '删除证书',
    message: `确认删除证书「${row.name}」吗？被入站引用时将无法删除。`,
    confirmButtonText: '删除',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  try {
    await deleteCert(row.id)
    message.success('证书已删除')
    await refresh()
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  }
}

defineExpose({ open, refresh })
</script>

<template>
  <u-dialog v-model="visible" title="证书管理" style="width: 760px; max-width: 96vw">
    <div class="cert-toolbar">
      <p class="cert-hint">TLS 类入站可引用面板存储证书（上传/粘贴 PEM）或服务器本地路径证书。</p>
      <u-button type="primary" @click="openCreate">新增证书</u-button>
    </div>
    <u-table :columns="columns" :data="certs" row-key="id" border :loading="loading">
      <template #column:mode="{ rowData }">
        <u-tag :type="rowData.mode === 'managed' ? 'success' : 'info'">
          {{ rowData.mode === 'managed' ? '面板存储' : '本地路径' }}
        </u-tag>
      </template>
      <template #column:paths="{ rowData }">
        <span class="cert-paths">{{ rowData.certPath }}<br />{{ rowData.keyPath }}</span>
      </template>
      <template #column:actions="{ rowData }">
        <u-button text size="small" type="primary" @click="openEdit(rowData)">编辑</u-button>
        <u-button text size="small" type="error" @click="handleDelete(rowData)">删除</u-button>
      </template>
    </u-table>

    <u-dialog v-model="formVisible" :title="editing ? '编辑证书' : '新增证书'" style="width: 640px; max-width: 96vw">
      <u-form ref="formRef" :model="form" :cols="1" label-width="92px">
        <u-input label="名称" field="name" placeholder="证书备注名" :rules="{ required: '请输入名称' }" />
        <u-select
          label="引用方式"
          field="mode"
          :options="modeOptions"
          :disabled="!!editing"
          :tips="editing ? '引用方式创建后不可变更' : undefined"
        />
        <template v-if="form.mode === 'managed'">
          <u-form-item label="证书 PEM" field="certPem" :rules="{ required: editing ? undefined : '请粘贴或上传证书 PEM' }">
            <u-textarea v-model="form.certPem" :rows="4" placeholder="-----BEGIN CERTIFICATE-----" />
            <u-button class="pem-upload" @click="certFileInput?.click()">上传证书文件</u-button>
            <input ref="certFileInput" type="file" accept=".pem,.crt,.cer,text/plain" hidden @change="pickPem('certPem', $event)" />
          </u-form-item>
          <u-form-item label="私钥 PEM" field="keyPem" :rules="{ required: editing ? undefined : '请粘贴或上传私钥 PEM' }">
            <u-textarea v-model="form.keyPem" :rows="4" placeholder="-----BEGIN PRIVATE KEY-----" />
            <u-button class="pem-upload" @click="keyFileInput?.click()">上传私钥文件</u-button>
            <input ref="keyFileInput" type="file" accept=".pem,.key,text/plain" hidden @change="pickPem('keyPem', $event)" />
          </u-form-item>
        </template>
        <template v-else>
          <u-input label="证书路径" field="certPath" placeholder="服务器上证书文件绝对路径" :rules="{ required: editing ? undefined : '请输入证书路径' }" />
          <u-input label="私钥路径" field="keyPath" placeholder="服务器上私钥文件绝对路径" :rules="{ required: editing ? undefined : '请输入私钥路径' }" />
        </template>
      </u-form>
      <p v-if="editing && form.mode === 'managed'" class="pem-hint">留空 PEM 内容表示仅改名；填写则需成对替换。</p>
      <template #footer="{ close }">
        <u-button text @click="close()">取消</u-button>
        <u-button type="primary" :loading="saving" @click="handleSave">保存</u-button>
      </template>
    </u-dialog>
  </u-dialog>
</template>

<style scoped>
.cert-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.cert-hint {
  margin: 0;
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}

.cert-paths {
  font-size: var(--u-font-size-main-small);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  word-break: break-all;
}

.pem-upload {
  margin-top: 6px;
}

.pem-hint {
  margin: 8px 0 0;
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}
</style>
