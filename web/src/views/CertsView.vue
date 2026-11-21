<!-- 证书页：TLS 证书的一级管理页。页顶说明卡解释证书是什么、什么时候需要、三种来源怎么选；
     列表展示来源、域名、到期时间与剩余天数（已过期醒目提示）；新增支持一键自签 / 上传粘贴 PEM / 引用服务器路径。
     入站 TLS 的「选择证书」下拉直接引用这里的面板存储证书；证书被入站占用时删除会被后端拒绝。 -->
<script setup lang="ts">
import { createCert, deleteCert, listCerts, selfSignCert, updateCert } from '../api/cert'
import type { CertKeyType, CertPayload, Certificate } from '../api/cert'
import {
  UAction,
  UActionGroup,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UDialog,
  UFilePicker,
  UForm,
  UFormItem,
  UInput,
  UMultiSelect,
  UNumberInput,
  USelect,
  UTable,
  UTag,
  UTextarea,
  defineTableColumns,
  message,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { nextTick, onMounted, reactive, shallowRef, useTemplateRef } from 'vue'

import EmptyState from '../components/EmptyState.vue'
import DialogFooter from '../components/DialogFooter.vue'
import FormGroupCard from '../components/FormGroupCard.vue'
import { confirmAction } from '../confirm-action'
import { dialogStyle } from '../dialog-style'
import { formatDateTime } from '../utils/format'

const certs = shallowRef<Certificate[]>([])
const loading = shallowRef(false)

const columns = defineTableColumns([
  { key: 'name', name: '名称', minWidth: 140 },
  { key: 'mode', name: '来源', width: 104, align: 'center' },
  { key: 'domains', name: '域名 / IP', minWidth: 200 },
  { key: 'expiry', name: '到期时间', minWidth: 210 },
  { key: 'status', name: '状态', width: 104, align: 'center' },
  { key: 'actions', name: '操作', width: 130, align: 'center' },
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

/** 到期状态标签：解析失败 / 已过期 / 即将到期（14 天内）/ 有效 */
function certStatus(row: Certificate): { label: string; type: 'danger' | 'warning' | 'success' } {
  if (row.certError || !row.notAfter) return { label: '解析失败', type: 'danger' }
  if (new Date(row.notAfter).getTime() < Date.now()) return { label: '已过期', type: 'danger' }
  if ((row.daysLeft ?? 0) <= 14) return { label: '即将到期', type: 'warning' }
  return { label: '有效', type: 'success' }
}

function isExpired(row: Certificate): boolean {
  return !!row.notAfter && new Date(row.notAfter).getTime() < Date.now()
}

/** 到期列文案：到期时间 + 剩余 / 已过期天数 */
function expiryText(row: Certificate): string {
  if (row.certError) return row.certError
  if (!row.notAfter) return '—'
  const base = formatDateTime(row.notAfter)
  const days = row.daysLeft
  if (days === null) return base
  return days >= 0 ? `${base}（剩 ${days} 天）` : `${base}（已过期 ${-days} 天）`
}

// ---------- 新增 / 编辑 ----------

/** 新增来源：self 一键自签 / managed 上传粘贴 PEM / path 引用服务器路径；编辑沿用记录的 mode */
type FormSource = 'self' | 'managed' | 'path'

function defaultCertForm() {
  return {
    name: '',
    source: 'self' as FormSource,
    domain: '',
    sans: [] as string[],
    days: 3650,
    keyType: 'p256' as CertKeyType,
    certPem: '',
    keyPem: '',
    certPath: '',
    keyPath: '',
  }
}

const formVisible = shallowRef(false)
const editing = shallowRef<Certificate | null>(null)
const saving = shallowRef(false)
const formRef = useTemplateRef<FormExposed>('formRef')
const form = reactive(defaultCertForm())

const sourceOptions = [
  { label: '一键自签（推荐自用）', value: 'self' },
  { label: '上传 / 粘贴 PEM', value: 'managed' },
  { label: '引用服务器路径', value: 'path' },
]

const keyTypeOptions = [
  { label: 'ECDSA P-256（推荐：体积小、握手快）', value: 'p256' },
  { label: 'RSA 2048（兼容老客户端）', value: 'rsa2048' },
  { label: 'Ed25519（最新，需 TLS 1.3）', value: 'ed25519' },
]

function openCreate(): void {
  editing.value = null
  Object.assign(form, defaultCertForm())
  formVisible.value = true
  void nextTick(() => formRef.value?.clearValidate())
}

/** 编辑沿用记录的引用方式（创建后不可变更）：自签证书按面板存储口径编辑 */
function openEdit(row: Certificate): void {
  editing.value = row
  Object.assign(form, defaultCertForm(), {
    name: row.name,
    source: row.mode,
    certPath: row.certPath,
    keyPath: row.keyPath,
  })
  formVisible.value = true
  void nextTick(() => formRef.value?.clearValidate())
}

/** 读取本地证书/私钥文件内容填入文本域 */
async function pickPem(target: 'certPem' | 'keyPem', files: File[]): Promise<void> {
  const file = files[0]
  if (file) form[target] = await file.text()
}

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    if (!editing.value && form.source === 'self') {
      await selfSignCert({
        ...(form.name ? { name: form.name } : {}),
        domain: form.domain.trim(),
        sans: form.sans.map((s) => s.trim()).filter(Boolean),
        days: form.days,
        keyType: form.keyType,
      })
      message.success('自签证书已生成，可在入站 TLS 中直接引用')
    } else {
      const payload: CertPayload =
        form.source === 'managed'
          ? {
              name: form.name,
              mode: 'managed',
              ...(form.certPem || form.keyPem ? { certPem: form.certPem, keyPem: form.keyPem } : {}),
            }
          : {
              name: form.name,
              mode: 'path',
              ...(form.certPath || form.keyPath ? { certPath: form.certPath, keyPath: form.keyPath } : {}),
            }
      if (editing.value) await updateCert(editing.value.id, payload)
      else await createCert(payload)
      message.success(editing.value ? '证书已更新' : '证书已添加')
    }
    formVisible.value = false
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: Certificate): Promise<void> {
  const ok = await confirmAction(
    '删除证书',
    `确认删除证书「${row.name}」吗？被入站 TLS 或 ECH 密钥引用时将无法删除。`,
    true,
    '删除',
  )
  if (!ok) return
  try {
    await deleteCert(row.id)
    message.success('证书已删除')
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
  }
}

onMounted(refresh)
</script>

<template>
  <!-- 页顶用途说明卡：什么是证书 / 什么时候需要 / 三种来源怎么选 -->
  <u-card size="small" class="intro-card">
    <u-card-header>证书是什么，什么时候需要</u-card-header>
    <u-card-content>
      <div>
        TLS 证书（证书 + 私钥）用于入站启用 TLS 时的身份证明与握手加密。
        当入站协议启用 TLS（Hysteria2 / TUIC / Trojan / VMess / AnyTLS 等）、客户端用域名连接时，就需要在这里准备一张证书供入站引用。
      </div>
      <ul class="intro-list">
        <li><strong>一键自签</strong>：填域名与有效期即可生成，由面板保管。适合自用：浏览器访问会提示「不受信任」，属正常现象，代理客户端不受影响。</li>
        <li><strong>上传 / 粘贴 PEM</strong>：已有正式证书（域名签发 / acme 工具导出）时交给面板保管，浏览器不再提示不受信任。</li>
        <li><strong>引用服务器路径</strong>：证书文件由外部工具（如 certbot）自动轮换时，面板只引用路径不接管文件。</li>
      </ul>
      <div class="intro-note">本面板不做 ACME 自动申请；入站 TLS 的「选择证书」下拉即引用这里的面板存储证书。</div>
    </u-card-content>
  </u-card>

  <u-card>
    <u-card-header>
      <div class="page-header">
        <span>证书管理</span>
        <div class="page-ops">
          <!-- 列表为空时主入口由空状态 CTA 承担，保持每屏 primary ≤1 -->
          <u-button v-if="certs.length" type="primary" @click="openCreate">新增证书</u-button>
        </div>
      </div>
    </u-card-header>
    <u-card-content>
      <u-table :columns="columns" :data="certs" row-key="id" stripe :loading="loading">
        <template #empty>
          <EmptyState
            v-if="!loading"
            title="暂无证书"
            description="还没有证书。自用可一键生成自签证书（填域名与有效期即可），有正式证书可上传或引用服务器路径"
            cta-text="新增证书"
            @cta="openCreate"
          />
        </template>
        <template #column:mode="{ rowData }">
          <u-tag :type="rowData.mode === 'managed' ? 'success' : 'info'">
            {{ rowData.mode === 'managed' ? '面板存储' : '本地路径' }}
          </u-tag>
        </template>
        <template #column:domains="{ rowData }">
          <span v-if="rowData.domains?.length" class="cell-domains">{{ rowData.domains.join('、') }}</span>
          <span v-else class="cell-dim">{{ rowData.certError ? '—' : rowData.certPath }}</span>
        </template>
        <template #column:expiry="{ rowData }">
          <span :class="{ 'cell-expired': isExpired(rowData) || rowData.certError }">
            {{ expiryText(rowData) }}
          </span>
        </template>
        <template #column:status="{ rowData }">
          <u-tag :type="certStatus(rowData).type">{{ certStatus(rowData).label }}</u-tag>
        </template>
        <template #column:actions="{ rowData }">
          <u-action-group>
            <u-action @run="openEdit(rowData)">编辑</u-action>
            <u-action type="danger" @run="handleDelete(rowData)">删除</u-action>
          </u-action-group>
        </template>
      </u-table>
    </u-card-content>
  </u-card>

  <u-dialog v-model="formVisible" :title="editing ? `编辑证书 · ${editing.name}` : '新增证书'" :style="dialogStyle(1120)">
    <!-- 内容超高时由 UDialog 自带滚动容器接管，header / footer 常驻 -->
    <u-form ref="formRef" :model="form" :cols="1" label-width="112px">
      <FormGroupCard title="基本信息">
        <u-form-item
          label="名称"
          field="name"
          :rules="form.source === 'self' && !editing ? undefined : { required: '请输入名称' }"
          :tips="form.source === 'self' && !editing ? '证书备注名，供入站 TLS 引用时识别；留空取主域名' : '证书备注名，供入站 TLS 引用时识别'"
        >
          <u-input v-model="form.name" placeholder="证书备注名" />
        </u-form-item>
        <u-form-item
          v-if="!editing"
          label="来源"
          field="source"
          tips="一键自签由面板生成并保管（适合自用）；上传/粘贴已有证书 PEM 交面板保管；服务器路径只引用不接管文件"
        >
          <u-select v-model="form.source" :options="sourceOptions" />
        </u-form-item>
      </FormGroupCard>

      <!-- 一键自签（仅新增） -->
      <FormGroupCard v-if="!editing && form.source === 'self'" title="自签参数">
        <u-form-item
          label="主域名"
          field="domain"
          :rules="{ required: '请输入主域名' }"
          tips="作为证书 CN 与首个 SAN；客户端连接入站时使用的域名需与之一致"
        >
          <u-input v-model="form.domain" placeholder="如 example.com" />
        </u-form-item>
        <u-form-item
          label="有效期"
          field="days"
          :rules="{ required: '请输入有效期天数' }"
          tips="自签证书需要手动轮换，自用建议一次给足（默认 10 年）；1-36500 天"
        >
          <u-number-input v-model="form.days" :min="1" :max="36500" placeholder="天数（默认 3650）" />
        </u-form-item>
        <u-form-item
          label="附加 SAN"
          field="sans"
          span="full"
          tips="额外覆盖的域名或 IP（如 www.example.com、10.0.0.8），可留空；输入后回车添加"
        >
          <u-multi-select v-model="form.sans" filterable creatable clearable placeholder="输入域名或 IP 后回车" />
        </u-form-item>
        <u-form-item label="密钥类型" field="keyType" tips="默认 ECDSA P-256：体积小、握手快；老客户端兼容性有问题时换 RSA 2048">
          <u-select v-model="form.keyType" :options="keyTypeOptions" />
        </u-form-item>
      </FormGroupCard>

      <!-- 上传/粘贴 PEM -->
      <FormGroupCard v-if="form.source === 'managed'" title="证书内容">
        <u-form-item
          label="证书 PEM"
          field="certPem"
          span="full"
          :rules="editing ? undefined : { required: '请粘贴或上传证书 PEM' }"
          tips="PEM 格式证书全文（-----BEGIN CERTIFICATE----- 开头）；编辑时留空表示仅改名"
        >
          <u-textarea v-model="form.certPem" :rows="4" placeholder="-----BEGIN CERTIFICATE-----" />
          <u-file-picker class="pem-upload" accept=".pem,.crt,.cer,text/plain" @pick="pickPem('certPem', $event)">
            <u-button>上传证书文件</u-button>
          </u-file-picker>
        </u-form-item>
        <u-form-item
          label="私钥 PEM"
          field="keyPem"
          span="full"
          :rules="editing ? undefined : { required: '请粘贴或上传私钥 PEM' }"
          tips="PEM 格式私钥全文（-----BEGIN PRIVATE KEY----- 开头）；填写则需与证书成对替换"
        >
          <u-textarea v-model="form.keyPem" :rows="4" placeholder="-----BEGIN PRIVATE KEY-----" />
          <u-file-picker class="pem-upload" accept=".pem,.key,text/plain" @pick="pickPem('keyPem', $event)">
            <u-button>上传私钥文件</u-button>
          </u-file-picker>
        </u-form-item>
      </FormGroupCard>

      <!-- 引用服务器路径 -->
      <FormGroupCard v-if="form.source === 'path'" title="文件路径">
        <u-form-item
          label="证书路径"
          field="certPath"
          span="full"
          :rules="editing ? undefined : { required: '请输入证书路径' }"
          tips="服务器上证书文件的绝对路径，内核进程需有读取权限"
        >
          <u-input v-model="form.certPath" placeholder="服务器上证书文件绝对路径" />
        </u-form-item>
        <u-form-item
          label="私钥路径"
          field="keyPath"
          span="full"
          :rules="editing ? undefined : { required: '请输入私钥路径' }"
          tips="服务器上私钥文件的绝对路径，与证书路径配套使用"
        >
          <u-input v-model="form.keyPath" placeholder="服务器上私钥文件绝对路径" />
        </u-form-item>
      </FormGroupCard>
    </u-form>
    <p v-if="editing && form.source === 'managed'" class="pem-hint">留空 PEM 内容表示仅改名；填写则需成对替换。</p>
    <template #footer="{ close }">
      <DialogFooter
        :loading="saving"
        :confirm-text="!editing && form.source === 'self' ? '生成' : '保存'"
        @cancel="close"
        @confirm="handleSave"
      />
    </template>
  </u-dialog>
</template>

<style scoped>
/* intro-card / intro-list / intro-note / page-header / page-ops 为 App.vue 全局类 */

.cell-domains {
  font-size: var(--u-font-size-main-small);
  word-break: break-all;
}

/* 域名 / 路径单元格：颜色取全局 cell-dim，这里补截断与小字号 */
.cell-dim {
  font-size: var(--u-font-size-main-small);
  word-break: break-all;
}

/* 已过期 / 解析失败的到期信息用错误色，与状态标签一起构成醒目提示 */
.cell-expired {
  color: var(--u-color-danger);
  font-size: var(--u-font-size-main-small);
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
