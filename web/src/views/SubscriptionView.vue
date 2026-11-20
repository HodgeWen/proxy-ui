<!-- 订阅用户（s-ui 风格）：为每台设备/每位使用者创建独立订阅链接。
     布局：页顶说明卡 + 左侧订阅用户列表（UTable 行点击选中）/ 右侧三种格式链接 + etiket 二维码，
     新建走 UDialog 弹窗表单；链接为面板同源子路径，反代/改端口自动适配 -->
<script setup lang="ts">
import { clipboard } from '@cat-kit/fe'
import { qrcode } from 'etiket/qr'
import { qrcodePNGDataURI } from 'etiket/png'
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UDialog,
  UForm,
  UFormItem,
  UInput,
  UTable,
  UTag,
  defineTableColumns,
  message,
  messageConfirm,
} from '@veltra/desktop'
import type { FormExposed, TableRow } from '@veltra/desktop'
import { QrCode, Token } from '@veltra/icons/normal'
import { computed, onMounted, reactive, shallowRef, useTemplateRef } from 'vue'

import EmptyState from '../components/EmptyState.vue'
import {
  createSubscriptionToken,
  deleteSubscriptionToken,
  getSubscription,
  resetSubscriptionToken,
  revokeSubscriptionToken,
  subscriptionLink,
} from '../api/subscription'
import type { SubscriptionToken } from '../api/subscription'

const users = shallowRef<SubscriptionToken[]>([])
const loading = shallowRef(false)
const selectedID = shallowRef(0)
const actingID = shallowRef(0)

/** 当前选中的订阅用户（默认选中第一个启用用户） */
const selected = computed(
  () => users.value.find((u) => u.id === selectedID.value) ?? users.value.find((u) => !u.revoked) ?? null,
)

const columns = defineTableColumns([
  { key: 'name', name: '名称', minWidth: 120 },
  { key: 'status', name: '状态', width: 88, align: 'center' },
  { key: 'createdAt', name: '创建时间', minWidth: 150 },
  { key: 'lastUsedAt', name: '最近使用', minWidth: 150 },
  { key: 'actions', name: '操作', width: 250, align: 'center' },
])

/** 当前展示二维码的订阅链接（自动适配格式） */
const link = computed(() => (selected.value ? subscriptionLink(selected.value.token) : ''))
const qrSVG = computed(() => (link.value ? qrcode(link.value, { size: 220, margin: 2 }) : ''))

/** 三种客户端格式的订阅链接 */
const linkFormats = computed(() => {
  if (!selected.value) return []
  const token = selected.value.token
  return [
    { label: '自动适配', link: subscriptionLink(token) },
    { label: 'sing-box', link: subscriptionLink(token, 'singbox') },
    { label: 'clash / mihomo', link: subscriptionLink(token, 'clash') },
  ]
})

async function refresh(): Promise<void> {
  loading.value = true
  try {
    users.value = await getSubscription()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    loading.value = false
  }
}

/** 点击行即选中，右侧展示该用户的链接与二维码 */
function handleRowClick(row: TableRow): void {
  selectedID.value = row.data.id
}

// ---------- 新建订阅用户 ----------

const createVisible = shallowRef(false)
const creating = shallowRef(false)
const createForm = reactive({ name: '' })
const createFormRef = useTemplateRef<FormExposed>('createForm')

function openCreate(): void {
  createForm.name = ''
  createVisible.value = true
}

async function handleCreate(): Promise<void> {
  if (!(await createFormRef.value?.validate())) return
  creating.value = true
  try {
    const created = await createSubscriptionToken(createForm.name.trim())
    createVisible.value = false
    await refresh()
    selectedID.value = created.id
    message.success('订阅用户已创建，点击其行即可查看订阅链接')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    creating.value = false
  }
}

// ---------- 列表操作 ----------

/** 危险操作（停用/重置/删除）二次确认 */
async function confirmAction(title: string, tip: string): Promise<boolean> {
  const action = await messageConfirm({
    title,
    message: tip,
    confirmButtonText: '确认',
    cancelButtonText: '取消',
  }).onClosed
  return action === 'confirm'
}

async function handleToggle(user: SubscriptionToken): Promise<void> {
  if (!user.revoked) {
    const ok = await confirmAction('停用订阅用户', `停用后「${user.name || '未命名'}」的客户端立即拉不到订阅，确认停用吗？`)
    if (!ok) return
  }
  actingID.value = user.id
  try {
    await revokeSubscriptionToken(user.id, !user.revoked)
    await refresh()
    message.success(user.revoked ? '订阅用户已启用' : '订阅用户已停用')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

async function handleReset(user: SubscriptionToken): Promise<void> {
  const ok = await confirmAction(
    '重置订阅链接',
    `重置将生成新链接，「${user.name || '未命名'}」的旧链接立即失效，需在客户端重新导入，确认继续吗？`,
  )
  if (!ok) return
  actingID.value = user.id
  try {
    await resetSubscriptionToken(user.id)
    await refresh()
    selectedID.value = user.id
    message.success('订阅链接已重置，请在客户端更新订阅链接')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

async function handleDelete(user: SubscriptionToken): Promise<void> {
  const ok = await confirmAction(
    '删除订阅用户',
    `删除后「${user.name || '未命名'}」的订阅链接立即失效且不可恢复，确认删除吗？`,
  )
  if (!ok) return
  actingID.value = user.id
  try {
    await deleteSubscriptionToken(user.id)
    await refresh()
    message.success('订阅用户已删除')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

async function handleCopy(text: string): Promise<void> {
  try {
    await clipboard.copy(text)
    message.success('已复制')
  } catch {
    message.error('复制失败，请手动复制')
  }
}

/** 下载当前订阅链接的 PNG 二维码 */
function handleDownloadPNG(): void {
  if (!link.value) return
  const a = document.createElement('a')
  a.href = qrcodePNGDataURI(link.value, { moduleSize: 12, margin: 2 })
  a.download = 'subscription-qr.png'
  a.click()
}

function formatTime(value: string | null): string {
  return value ? new Date(value).toLocaleString() : '从未使用'
}

onMounted(refresh)
</script>

<template>
  <!-- 页顶说明卡：解释订阅用户模型 -->
  <u-card class="intro-card">
    <u-card-header>订阅用户</u-card-header>
    <u-card-content>
      <p class="intro-text">
        为每台设备或每位使用者创建一个「订阅用户」，各自持有独立的订阅链接，互不影响。
        链接就是面板的同源子路径，客户端从访问面板的同一地址拉取订阅，反向代理或修改端口后自动适配，无需配置服务器地址。
      </p>
      <ul class="intro-tips">
        <li>停用用户后，其客户端立即拉不到订阅；重新启用即恢复</li>
        <li>重置链接后旧链接立即失效，需在客户端重新导入新链接</li>
      </ul>
    </u-card-content>
  </u-card>

  <div class="sub-grid">
    <!-- 左/主区：订阅用户列表，行点击选中 -->
    <u-card>
      <u-card-header>
        <div class="page-header">
          <span>订阅用户列表</span>
          <u-button v-if="users.length" type="primary" @click="openCreate">新建订阅用户</u-button>
        </div>
      </u-card-header>
      <u-card-content>
        <u-table
          :columns="columns"
          :data="users"
          row-key="id"
          border
          stripe
          :loading="loading"
          highlight-current
          @row-click="handleRowClick"
        >
          <template #empty>
            <EmptyState
              v-if="!loading"
              :icon="Token"
              title="尚无订阅用户"
              description="为每台设备或每位使用者创建一个订阅用户，即可获得供客户端订阅的链接与二维码"
              cta-text="新建订阅用户"
              @cta="openCreate"
            />
          </template>
          <template #column:name="{ rowData }">
            <span :class="{ 'name-stopped': rowData.revoked }">{{ rowData.name || '未命名' }}</span>
          </template>
          <template #column:status="{ rowData }">
            <u-tag :type="rowData.revoked ? 'danger' : 'success'">{{ rowData.revoked ? '已停用' : '启用' }}</u-tag>
          </template>
          <template #column:createdAt="{ rowData }">
            {{ formatTime(rowData.createdAt) }}
          </template>
          <template #column:lastUsedAt="{ rowData }">
            {{ formatTime(rowData.lastUsedAt) }}
          </template>
          <template #column:actions="{ rowData }">
            <u-button text size="small" type="primary" :disabled="rowData.revoked" @click="handleCopy(subscriptionLink(rowData.token))">
              复制订阅
            </u-button>
            <u-button text size="small" :loading="actingID === rowData.id" @click="handleToggle(rowData)">
              {{ rowData.revoked ? '启用' : '停用' }}
            </u-button>
            <u-button text size="small" :loading="actingID === rowData.id" @click="handleReset(rowData)">重置链接</u-button>
            <u-button text size="small" type="error" :loading="actingID === rowData.id" @click="handleDelete(rowData)">删除</u-button>
          </template>
        </u-table>
      </u-card-content>
    </u-card>

    <!-- 右/侧区：选中用户的订阅链接与二维码 -->
    <u-card class="link-card">
      <u-card-header>订阅链接与二维码{{ selected?.name ? ` · ${selected.name}` : '' }}</u-card-header>
      <u-card-content>
        <EmptyState
          v-if="!selected"
          :icon="QrCode"
          title="未选中订阅用户"
          description="点击左侧列表中的订阅用户，此处展示各客户端格式的订阅链接与二维码"
        />
        <template v-else>
          <u-tag v-if="selected.revoked" type="danger">已停用：客户端暂时拉不到订阅</u-tag>
          <div class="links">
            <div v-for="fmt in linkFormats" :key="fmt.label" class="link-item">
              <div class="link-head">
                <span class="link-label">{{ fmt.label }}</span>
                <u-button size="small" @click="handleCopy(fmt.link)">复制</u-button>
              </div>
              <code class="link-value">{{ fmt.link }}</code>
            </div>
          </div>
          <div class="qr">
            <!-- etiket 生成的订阅二维码 SVG（内容为面板自身链接，可信） -->
            <!-- eslint-disable-next-line vue/no-v-html -->
            <div class="qr-svg" v-html="qrSVG"></div>
            <div class="qr-side">
              <div class="qr-tip">客户端扫描导入订阅，格式按客户端 UA 自动适配</div>
              <u-button size="small" @click="handleDownloadPNG">下载 PNG</u-button>
            </div>
          </div>
        </template>
      </u-card-content>
    </u-card>
  </div>

  <!-- 新建订阅用户：名称必填 -->
  <u-dialog v-model="createVisible" title="新建订阅用户" style="width: 480px; max-width: 96vw">
    <u-form ref="createForm" :model="createForm" :cols="1" label-width="72px">
      <u-form-item label="名称" field="name" :rules="{ required: '请输入名称' }" tips="用于辨识设备或使用者，如「我的手机」">
        <u-input v-model="createForm.name" placeholder="如：我的手机" clearable @keyup.enter="handleCreate" />
      </u-form-item>
    </u-form>
    <template #footer="{ close }">
      <u-button text @click="close()">取消</u-button>
      <u-button type="primary" :loading="creating" @click="handleCreate">创建</u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
.intro-card {
  margin-bottom: 16px;
}

.intro-text {
  margin: 0;
  color: var(--u-text-color-main);
}

.intro-tips {
  margin: 8px 0 0;
  padding-left: 18px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.intro-tips li {
  margin-top: 2px;
}

/* 订阅用户列表与链接/二维码双栏铺满页面宽度，窄屏退化为单列 */
.sub-grid {
  display: grid;
  grid-template-columns: minmax(0, 3fr) minmax(0, 2fr);
  gap: 16px;
  align-items: stretch;
}

@media (max-width: 900px) {
  .sub-grid {
    grid-template-columns: 1fr;
  }
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex: 1;
}

.name-stopped {
  color: var(--u-text-color-second);
}

.link-card {
  min-width: 0;
}

.links {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.link-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.link-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.link-label {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.link-value {
  font-size: var(--u-font-size-main-small);
  word-break: break-all;
  padding: 6px 10px;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-small);
}

.qr {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--u-border-color);
}

.qr-svg {
  width: 220px;
  height: 220px;
  flex-shrink: 0;
}

.qr-svg :deep(svg) {
  width: 100%;
  height: 100%;
}

.qr-side {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.qr-tip {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}
</style>
