<!-- 订阅用户：一位使用者（或一台设备）一个订阅链接 + 独立配额/到期。
     勾选入站 = 在该入站里为这个人建一个用户身份（共用入站端口、各自凭据），流量按用户归属，
     配额用尽 / 到期 / 停用只暂停这个人的节点。链接与二维码收进弹窗，页面只留列表与配额管理 -->
<script setup lang="ts">
import {
  UAction,
  UActionGroup,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UDatePicker,
  UDialog,
  UForm,
  UFormItem,
  UInput,
  UMultiSelect,
  UNumberInput,
  USelect,
  USwitch,
  UTable,
  UTag,
  UTimePicker,
  defineTableColumns,
  message,
  messageConfirm,
  vLoading,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { UserGroup } from '@veltra/icons/normal'
import { computed, onMounted, reactive, shallowRef, useTemplateRef } from 'vue'

import EmptyState from '../components/EmptyState.vue'
import SubscriptionLinksDialog from '../components/SubscriptionLinksDialog.vue'
import { listInbounds } from '../api/inbound'
import type { Inbound } from '../api/inbound'
import {
  createSubscriptionToken,
  deleteSubscriptionToken,
  formatBytes,
  getSubscription,
  resetSubscriptionToken,
  resetSubscriptionTraffic,
  revokeSubscriptionToken,
  updateSubscriptionToken,
} from '../api/subscription'
import type { SubscriptionPayload, SubscriptionToken } from '../api/subscription'
import { protocolDef } from '../inbound-schema'

// ---------- 列表 ----------

const users = shallowRef<SubscriptionToken[]>([])
const loading = shallowRef(false)
const actingID = shallowRef(0)

const columns = defineTableColumns([
  { key: 'name', name: '名称', minWidth: 140 },
  { key: 'status', name: '状态', width: 96, align: 'center' },
  { key: 'usage', name: '用量', minWidth: 200 },
  { key: 'nodes', name: '节点', minWidth: 200 },
  { key: 'expiresAt', name: '到期时间', minWidth: 150 },
  { key: 'lastUsedAt', name: '最近使用', minWidth: 150 },
  // 订阅链接是订阅用户专属动作，单独成列；通用操作列固定在右侧
  { key: 'links', name: '订阅', width: 96, align: 'center' },
  { key: 'actions', name: '操作', width: 160, align: 'center', fixed: 'right' },
])

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

/** 状态标签：停用优先于到期，到期优先于超额 */
function statusOf(user: SubscriptionToken): { label: string; type: 'info' | 'warning' | 'danger' | 'success' } {
  if (user.revoked) return { label: '已停用', type: 'info' }
  if (user.expired) return { label: '已到期', type: 'warning' }
  if (user.overQuota) return { label: '流量用尽', type: 'danger' }
  return { label: '启用', type: 'success' }
}

/** 配额使用百分比（配额为 0 表示不限） */
function usagePercent(user: SubscriptionToken): number {
  if (!user.trafficLimitBytes) return 0
  return Math.min(100, Math.round((user.usedBytes / user.trafficLimitBytes) * 100))
}

/** 节点概况：数量 + 去重后的协议名 */
function nodeSummary(user: SubscriptionToken): string {
  const protocols = [...new Set(user.nodes.map((n) => protocolShortName(n.protocol)))]
  return `${user.nodes.length} 个 · ${protocols.join('、')}`
}

function protocolName(protocol: string): string {
  return protocolDef(protocol)?.name ?? protocol
}

/** 协议族短名：节点列放不下「VLESS + Reality」「TUIC v5」「Shadowsocks-2022」，只留协议族 */
function protocolShortName(protocol: string): string {
  return protocolName(protocol)
    .replace(/-\d[\w.]*$/, '')
    .split(' ')[0]
}

function formatTime(value: string): string {
  return new Date(value).toLocaleString()
}

// ---------- 订阅链接弹窗 ----------

const linksVisible = shallowRef(false)
const linksUser = shallowRef<SubscriptionToken | null>(null)

function openLinks(user: SubscriptionToken): void {
  linksUser.value = user
  linksVisible.value = true
}

// ---------- 新建 / 编辑 ----------

/** 配额单位；'none' 表示不限 */
type QuotaUnit = 'none' | 'MB' | 'GB' | 'TB'

const QUOTA_UNITS: { label: string; value: QuotaUnit }[] = [
  { label: '不限', value: 'none' },
  { label: 'MB', value: 'MB' },
  { label: 'GB', value: 'GB' },
  { label: 'TB', value: 'TB' },
]

const UNIT_BYTES: Record<Exclude<QuotaUnit, 'none'>, number> = {
  MB: 1024 ** 2,
  GB: 1024 ** 3,
  TB: 1024 ** 4,
}

const editorVisible = shallowRef(false)
const saving = shallowRef(false)
const editingId = shallowRef(0)
const editorFormRef = useTemplateRef<FormExposed>('editorForm')
const inbounds = shallowRef<Inbound[]>([])

const form = reactive({
  name: '',
  quotaUnit: 'none' as QuotaUnit,
  quotaValue: undefined as number | undefined,
  /** 永久有效（关闭时用 expiry 拼到期时刻） */
  permanent: true,
  expiry: { date: undefined as string | undefined, time: undefined as string | undefined },
  nodeInbounds: [] as number[],
})

/** 可勾选的入站：UMultiSelect 不支持逐项禁用，停用的入站不进选项；
     但该用户已勾选的（编辑回显）仍列出并标注「已停用」，否则管理员取消不掉这个节点 */
const inboundOptions = computed(() =>
  inbounds.value
    .filter((t) => t.enabled || form.nodeInbounds.includes(t.id))
    .map((t) => ({
      label: `${t.name}（${protocolName(t.protocol)}，端口 ${t.listenPort}）${t.enabled ? '' : ' · 已停用'}`,
      value: t.id,
    })),
)
/** 未勾选且已停用的入站：不可勾选，列出来让管理员知道为什么没有它 */
const stoppedInbounds = computed(() =>
  inbounds.value.filter((t) => !t.enabled && !form.nodeInbounds.includes(t.id)),
)
const hasEnabledInbound = computed(() => inbounds.value.some((t) => t.enabled))

/** 切到「不限」时清掉数值，避免残留数字被误读成仍在限制用量 */
function handleQuotaUnitChange(unit: QuotaUnit): void {
  if (unit === 'none') form.quotaValue = undefined
}

const quotaRules = {
  validator: (value: number | undefined) =>
    form.quotaUnit !== 'none' && !value ? '请输入配额数值，或把单位切回「不限」' : '',
}

const expiryRules = {
  validator: (value: string | undefined) =>
    form.permanent || value ? '' : '请选择到期日期，或切换为「永久有效」',
}

/** 字节数拆回「数值 + 单位」，便于编辑回显 */
function splitQuota(bytes: number): { value: number | undefined; unit: QuotaUnit } {
  if (!bytes) return { value: undefined, unit: 'none' }
  if (bytes % UNIT_BYTES.TB === 0) return { value: bytes / UNIT_BYTES.TB, unit: 'TB' }
  if (bytes % UNIT_BYTES.GB === 0) return { value: bytes / UNIT_BYTES.GB, unit: 'GB' }
  // 不足 1 GB 的零头按 MB 回显；不足 1 MB 时保留两位小数，避免回显成 0 后校验不过
  return { value: Math.max(0.01, Math.round((bytes / UNIT_BYTES.MB) * 100) / 100), unit: 'MB' }
}

/** 表单到期时刻拆成日期 + 时间（本地时区） */
function splitDateTime(iso: string): { date: string; time: string } {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return {
    date: `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`,
    time: `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`,
  }
}

function quotaBytes(): number {
  if (form.quotaUnit === 'none') return 0
  return Math.round((form.quotaValue ?? 0) * UNIT_BYTES[form.quotaUnit])
}

/** 到期时刻转 RFC3339；未选日期时为 null */
function expiryISO(): string | null {
  const { date, time } = form.expiry
  return date ? new Date(`${date}T${time || '00:00:00'}`).toISOString() : null
}

async function openEditor(user: SubscriptionToken | null): Promise<void> {
  editingId.value = user?.id ?? 0
  const quota = splitQuota(user?.trafficLimitBytes ?? 0)
  const expiry = user?.expiresAt ? splitDateTime(user.expiresAt) : undefined
  form.name = user?.name ?? ''
  form.quotaUnit = quota.unit
  form.quotaValue = quota.value
  form.permanent = !expiry
  form.expiry = { date: expiry?.date, time: expiry?.time }
  // 已勾选的入站按节点的 inboundId 回显
  form.nodeInbounds = user ? user.nodes.map((n) => n.inboundId) : []
  editorVisible.value = true
  try {
    inbounds.value = await listInbounds()
  } catch {
    // 错误已由请求封装统一提示，入站列表为空时弹窗内会提示先去创建
  }
}

async function handleSave(): Promise<void> {
  if (!(await editorFormRef.value?.validate())) return
  saving.value = true
  const editing = editingId.value > 0
  try {
    const payload: SubscriptionPayload = {
      name: form.name.trim(),
      trafficLimitBytes: quotaBytes(),
      expiresAt: form.permanent ? null : expiryISO(),
      // 永久有效时显式清空到期时间（后端按 clearExpiry 优先处理）
      clearExpiry: form.permanent,
      nodeInbounds: form.nodeInbounds,
    }
    if (editing) await updateSubscriptionToken(editingId.value, payload)
    else await createSubscriptionToken(payload)
    editorVisible.value = false
    await refresh()
    message.success(editing ? '订阅用户已更新' : '订阅用户已创建，可在列表中查看其订阅链接')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

// ---------- 列表操作 ----------

async function confirmAction(title: string, tip: string, danger = false): Promise<boolean> {
  const action = await messageConfirm({
    title,
    message: tip,
    confirmButtonText: '确认',
    confirmButtonType: danger ? 'danger' : 'primary',
    cancelButtonText: '取消',
  }).onClosed
  return action === 'confirm'
}

async function handleToggle(user: SubscriptionToken): Promise<void> {
  actingID.value = user.id
  try {
    await revokeSubscriptionToken(user.id, !user.revoked)
    await refresh()
    message.success(user.revoked ? '订阅用户已启用' : '订阅用户已停用，其客户端立即拉不到订阅')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

/** 重置链接会换令牌，旧链接立即失效，必须说清后果 */
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
    message.success('订阅链接已重置，请在客户端更新订阅')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

async function handleResetTraffic(user: SubscriptionToken): Promise<void> {
  actingID.value = user.id
  try {
    await resetSubscriptionTraffic(user.id)
    await refresh()
    message.success('该用户的流量统计已清零，因超额暂停的节点随之恢复')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

/** 删除会连带节点与累计流量，必须说清后果 */
async function handleDelete(user: SubscriptionToken): Promise<void> {
  const ok = await confirmAction(
    '删除订阅用户',
    `删除后「${user.name || '未命名'}」的订阅链接立即失效，其在各入站里的 ${user.nodes.length} 个用户身份与累计流量一并删除且不可恢复，确认删除吗？`,
    true,
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

onMounted(refresh)
</script>

<template>
  <u-card>
    <u-card-header>
      <div class="page-header">
        <span>订阅用户</span>
        <!-- 列表为空时主入口由空状态 CTA 承担，保持每屏 primary ≤1 -->
        <u-button v-if="users.length" type="primary" @click="openEditor(null)">新建订阅用户</u-button>
      </div>
    </u-card-header>
    <u-card-content>
      <p class="card-desc">
        一位使用者（或一台设备）就是一个「订阅用户」，只有一条订阅链接，配额与到期时间都记在这条链接上。
        勾选入站即在该入站里为这个人创建一个用户身份：<b>同一入站下的节点共用该入站的端口，但凭据各自独立</b>。
        流量按用户归属统计，所以配额用尽、到期或停用时只暂停这个人的节点，同端口的其他人不受影响。
      </p>
      <div v-loading="loading" class="table-wrap">
        <u-table :columns="columns" :data="users" row-key="id" border>
          <template #empty>
            <EmptyState
              v-if="!loading"
              :icon="UserGroup"
              title="尚无订阅用户"
              description="为每位使用者或每台设备创建一个订阅用户，勾选入站后即在该入站里得到独立凭据的节点与订阅链接"
              cta-text="新建订阅用户"
              @cta="openEditor(null)"
            />
          </template>
          <template #column:name="{ rowData }">
            <span :class="{ 'cell-dim': rowData.revoked }">{{ rowData.name || '未命名' }}</span>
          </template>
          <template #column:status="{ rowData }">
            <u-tag :type="statusOf(rowData).type">{{ statusOf(rowData).label }}</u-tag>
          </template>
          <template #column:usage="{ rowData }">
            <div class="usage">
              <div class="usage-head">
                <span>
                  {{ formatBytes(rowData.usedBytes) }} /
                  {{ rowData.trafficLimitBytes ? formatBytes(rowData.trafficLimitBytes) : '不限' }}
                </span>
                <span v-if="rowData.trafficLimitBytes" class="cell-dim">{{ usagePercent(rowData) }}%</span>
              </div>
              <div v-if="rowData.trafficLimitBytes" class="usage-bar">
                <div
                  class="usage-bar-fill"
                  :class="{ 'is-over': rowData.overQuota }"
                  :style="{ width: `${usagePercent(rowData)}%` }"
                ></div>
              </div>
            </div>
          </template>
          <template #column:nodes="{ rowData }">
            <span v-if="rowData.nodes.length">{{ nodeSummary(rowData) }}</span>
            <span v-else class="cell-dim">未勾选入站</span>
          </template>
          <template #column:expiresAt="{ rowData }">
            {{ rowData.expiresAt ? formatTime(rowData.expiresAt) : '永久' }}
          </template>
          <template #column:lastUsedAt="{ rowData }">
            <span :class="{ 'cell-dim': !rowData.lastUsedAt }">
              {{ rowData.lastUsedAt ? formatTime(rowData.lastUsedAt) : '从未使用' }}
            </span>
          </template>
          <template #column:links="{ rowData }">
            <u-button text type="primary" size="small" @click="openLinks(rowData)">查看订阅</u-button>
          </template>
          <template #column:actions="{ rowData }">
            <!-- 5 个动作超出 max=3：编辑/启停内联，其余收进「更多」下拉 -->
            <u-action-group :max="3" :loading="actingID === rowData.id">
              <u-action @run="openEditor(rowData)">编辑</u-action>
              <!-- 停用会让客户端立刻断供：气泡确认即可；清零流量同样在气泡里确认 -->
              <u-action :need-confirm="!rowData.revoked" @run="handleToggle(rowData)">
                {{ rowData.revoked ? '启用' : '停用' }}
              </u-action>
              <!-- 重置链接会换令牌、删除会连节点与流量一起删，走弹窗说清后果 -->
              <u-action @run="handleReset(rowData)">重置链接</u-action>
              <u-action need-confirm @run="handleResetTraffic(rowData)">清零流量</u-action>
              <u-action type="danger" @run="handleDelete(rowData)">删除</u-action>
            </u-action-group>
          </template>
        </u-table>
      </div>
    </u-card-content>
  </u-card>

  <!-- 新建 / 编辑：名称、配额、到期、勾选的入站 -->
  <u-dialog
    v-model="editorVisible"
    :title="editingId ? '编辑订阅用户' : '新建订阅用户'"
    style="width: 620px; max-width: 96vw"
  >
    <u-form ref="editorForm" :model="form" :cols="1" label-width="88px">
      <u-input
        label="名称"
        field="name"
        placeholder="如：我的手机"
        tips="用于辨识设备或使用者；入站里的用户名由「名称-入站名」生成"
        :rules="{ required: '请输入名称' }"
      />
      <u-form-item
        label="流量配额"
        field="quotaValue"
        :rules="quotaRules"
        tips="上+下行累计用量达到配额后，该用户全部节点自动暂停；清零流量后恢复"
      >
        <div class="field-row">
          <u-number-input
            v-model="form.quotaValue"
            class="field-grow"
            :min="0"
            :step="1"
            :disabled="form.quotaUnit === 'none'"
            placeholder="不限"
          />
          <u-select
            v-model="form.quotaUnit"
            class="field-unit"
            :options="QUOTA_UNITS"
            @update:model-value="handleQuotaUnitChange"
          />
        </div>
      </u-form-item>
      <u-form-item label="到期时间" field="expiry.date" :rules="expiryRules">
        <div class="field-row">
          <u-switch v-model="form.permanent" active-text="永久有效" />
          <template v-if="!form.permanent">
            <u-date-picker v-model="form.expiry.date" class="field-grow" placeholder="到期日期" />
            <u-time-picker v-model="form.expiry.time" class="field-time" placeholder="时刻" />
          </template>
        </div>
      </u-form-item>
      <u-multi-select
        label="勾选入站"
        field="nodeInbounds"
        :options="inboundOptions"
        placeholder="选择该用户可用的入站"
        filterable
      />
      <p class="form-hint">
        勾选或取消入站，会在对应入站里创建 / 删除该用户的身份，节点随之增减；这些改动需到「入站」页点「应用配置」后生效。
        只改配额或到期时间时后端会自动应用，并且只暂停 / 恢复节点，不影响链接与端口。
      </p>
      <p v-if="!hasEnabledInbound" class="form-hint is-warning">
        暂无启用的入站，请先到「入站」页创建并启用入站，再回来为该用户勾选节点。
      </p>
      <p v-if="stoppedInbounds.length" class="form-hint">
        已停用、不可勾选的入站：{{ stoppedInbounds.map((t) => t.name).join('、') }}
      </p>
    </u-form>

    <template #footer="{ close }">
      <u-button text @click="close()">取消</u-button>
      <u-button type="primary" :loading="saving" @click="handleSave">保存</u-button>
    </template>
  </u-dialog>

  <SubscriptionLinksDialog v-model:visible="linksVisible" :token="linksUser" />
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex: 1;
}

.table-wrap {
  position: relative;
  min-height: 120px;
}

.card-desc {
  margin: 0 0 12px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.cell-dim {
  color: var(--u-text-color-assist);
}

.usage {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.usage-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: var(--u-font-size-main-small);
}

.usage-bar {
  height: 4px;
  border-radius: 2px;
  background-color: var(--u-bg-color-hover);
  overflow: hidden;
}

.usage-bar-fill {
  height: 100%;
  background-color: var(--u-color-primary);
  transition: width var(--u-transition-fast) var(--u-transition-ease-out);
}

.usage-bar-fill.is-over {
  background-color: var(--u-color-danger);
}

.field-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.field-grow {
  flex: 1;
  min-width: 0;
}

.field-unit {
  width: 96px;
  flex-shrink: 0;
}

.field-time {
  width: 120px;
  flex-shrink: 0;
}

.form-hint {
  margin: 0 0 8px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.form-hint.is-warning {
  color: var(--u-color-warning);
}
</style>
