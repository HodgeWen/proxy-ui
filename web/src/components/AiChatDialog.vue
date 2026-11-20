<!-- AI 助手弹框：顶栏按钮打开的近全屏 UDialog，内部承载 UAiChat；
     transport 元数据与工具来自后端（api-key 只存服务端，对话经面板代理流式转发），
     供应商/模型与推理等级选择由组件原生选择器承担；
     关闭弹框即卸载内容、重置当前对话；无可用供应商时给出直达设置页的引导 -->
<script setup lang="ts">
import type { ChatTool, OpenAITransport } from '@veltra/ai'
import { UAiChat, createOpenAITransport } from '@veltra/ai'
import { UButton, UDialog, message } from '@veltra/desktop'
import { AiChat, TriangleAlert } from '@veltra/icons/normal'
import { shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'

import { callAITool, fetchAITools, fetchAITransport } from '../api/ai'
import type { AiToolSchema } from '../api/ai'
import { dialogStyle } from '../dialog-style'
import EmptyState from './EmptyState.vue'

const visible = defineModel<boolean>({ default: false })

const router = useRouter()

const loading = shallowRef(false)
/** 无可用供应商（未配置或均无模型）时展示引导 */
const notReady = shallowRef(false)
const configError = shallowRef('')
const transport = shallowRef<OpenAITransport | null>(null)
const model = shallowRef('')
const tools = shallowRef<ChatTool[]>([])

const systemPrompt = [
  '你是 proxy-ui（sing-box 代理面板）的 AI 助手，帮助用户查询与修改面板配置、解答代理相关问题。',
  '可用面板工具覆盖：入站与入站用户、出站、TLS 证书（含一键自签）、路由规则与规则集、策略组、订阅用户、DNS、系统优化（BBR）与流量统计的查询与修改。',
  '注意：',
  '- 修改类工具（创建/更新/删除）执行前会请求用户确认；',
  '- 任何配置修改保存后内核都会立即按新配置重建并生效，不存在也不需要单独的「应用配置」操作；新配置启动失败时内核自动回到上一份可用配置，并把原因回给你；',
  '- 内核随面板自动运行，只能查看状态与日志，不存在启停或更换内核的操作；',
  '- 涉及端口、密码、协议选择等具体取值而用户未给定时，先用一句话向用户确认再执行；',
  '- 回答使用中文，工具结果中的数据如实转述，不要编造。',
].join('\n')

/** 把后端工具描述包装为对话工具（执行回落到后端业务模块） */
function toChatTool(schema: AiToolSchema): ChatTool {
  return {
    name: schema.name,
    description: schema.description,
    parameters: schema.parameters,
    needsConfirm: schema.needsConfirm,
    execute: (args) => callAITool(schema.name, (args ?? {}) as Record<string, unknown>),
  }
}

async function init(): Promise<void> {
  loading.value = true
  configError.value = ''
  try {
    const { providers } = await fetchAITransport()
    // createOpenAITransport 要求每个供应商至少 1 个模型，无模型的供应商不参与对话
    const usable = providers.filter((provider) => provider.models.length > 0)
    if (!usable.length) {
      notReady.value = true
      transport.value = null
      return
    }
    notReady.value = false
    const t = createOpenAITransport({ providers: usable })
    transport.value = t
    model.value = t.defaultModel
    tools.value = (await fetchAITools()).map(toChatTool)
  } catch (error) {
    configError.value = error instanceof Error ? error.message : String(error)
  } finally {
    loading.value = false
  }
}

/** 每次打开都重新拉取配置（设置页可能刚改过供应商）；关闭即卸载内容、重置会话 */
watch(visible, (open) => {
  if (open) void init()
})

function gotoSettings(): void {
  visible.value = false
  router.push('/settings')
}

function retry(): void {
  message.info('正在重新加载 AI 配置')
  void init()
}
</script>

<template>
  <u-dialog v-model="visible" title="AI 助手" :style="dialogStyle(1280, 94)">
    <template #default="{ maximized }">
      <!-- 未就绪三态（加载中/加载失败/无可用供应商）与对话本体占满弹框内容区 -->
      <div
        v-if="loading || configError || notReady || !transport"
        class="ai-dialog-status"
        :style="{ height: maximized ? '100%' : 'calc(90vh - 82px)' }"
      >
        <EmptyState v-if="loading" title="正在加载 AI 配置…" />
        <EmptyState
          v-else-if="configError"
          :icon="TriangleAlert"
          title="AI 配置加载失败"
          :description="configError"
          cta-text="重试"
          @cta="retry"
        />
        <template v-else>
          <EmptyState
            :icon="AiChat"
            title="尚未配置可用的 AI 供应商"
            description="请在设置页新增 OpenAI 兼容供应商（接口地址 + API Key）并添加模型后开始对话"
            cta-text="前往设置配置 AI 供应商"
            @cta="gotoSettings"
          />
          <div class="ai-dialog-ops">
            <u-button @click="retry">我已配置，刷新</u-button>
          </div>
        </template>
      </div>
      <!-- 组件根节点 height: 100%，高度由外层显式给出；内部自带消息滚动 -->
      <u-ai-chat
        v-else
        :transport="transport"
        :tools="tools"
        :models="transport.models"
        v-model:model="model"
        :system-prompt="systemPrompt"
        :max-tool-rounds="12"
        :style="{ height: maximized ? '100%' : 'calc(90vh - 82px)' }"
      />
    </template>
  </u-dialog>
</template>

<style scoped>
/* 状态区在弹框内居中；对话本体自身管理布局，这里只铺高度 */
.ai-dialog-status {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.ai-dialog-ops {
  margin-top: 16px;
}
</style>
