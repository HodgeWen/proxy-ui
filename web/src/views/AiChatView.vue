<!-- AI 助手：@veltra/ai 流式对话；面板接口注册为工具（经 /api/ai 代理与执行），未配置供应商时给出配置引导 -->
<script setup lang="ts">
import type { ChatTool, OpenAITransport } from '@veltra/ai'
import { UAiChat, createOpenAITransport } from '@veltra/ai'
import { UButton, UCard, UCardContent, UCardHeader, message } from '@veltra/desktop'
import { AiChat, TriangleAlert } from '@veltra/icons/normal'
import { computed, onMounted, shallowRef } from 'vue'
import { useRouter } from 'vue-router'

import { callAITool, fetchAIConfig, fetchAITools } from '../api/ai'
import type { AiToolSchema } from '../api/ai'
import EmptyState from '../components/EmptyState.vue'

const router = useRouter()

const loading = shallowRef(true)
const configured = shallowRef(false)
const configError = shallowRef('')
const transport = shallowRef<OpenAITransport | null>(null)
const model = shallowRef('')
const tools = shallowRef<ChatTool[]>([])

/** 对话页就绪（供应商已配置且 transport 已构建） */
const ready = computed(() => configured.value && transport.value !== null)

/** 模型选项（来自后端配置的单个模型） */
const modelOptions = computed(() => transport.value?.models ?? [])

const systemPrompt = [
  '你是 proxy-ui（sing-box 代理面板）的 AI 助手，帮助用户查询与修改面板配置、解答代理相关问题。',
  '可用面板工具覆盖：入站、出站、全局规则、策略组、订阅令牌、内核控制、全局设置与流量统计的查询与修改。',
  '注意：',
  '- 修改类工具（创建/更新/删除、内核控制、应用配置）执行前会请求用户确认；',
  '- 入站/出站/规则等配置修改后不会立即生效，需提醒用户确认后调用 apply_config 重新生成并应用内核配置；',
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
    const config = await fetchAIConfig()
    if (!config.configured) {
      configured.value = false
      return
    }
    // 相对路径指向面板代理，api-key 只存在服务端；chat 模式直接 POST endpoint
    const t = createOpenAITransport({
      providers: [
        {
          id: 'panel',
          label: '面板 AI',
          endpoint: '/api/ai/chat',
          mode: 'chat',
          models: [{ id: config.model, label: config.model }],
        },
      ],
    })
    transport.value = t
    model.value = t.defaultModel
    configured.value = true
    tools.value = (await fetchAITools()).map(toChatTool)
  } catch (error) {
    configError.value = error instanceof Error ? error.message : String(error)
  } finally {
    loading.value = false
  }
}

function gotoSettings(): void {
  router.push('/settings')
}

function retry(): void {
  message.info('正在重新加载 AI 配置')
  void init()
}

onMounted(init)
</script>

<template>
  <div class="ai-page">
    <!-- 未就绪三态（加载中/加载失败/未配置）共用一张居中状态卡 -->
    <div v-if="!ready" class="ai-status-wrap">
      <u-card class="ai-status-card">
        <u-card-header>AI 助手</u-card-header>
        <u-card-content>
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
              title="尚未配置 AI 供应商"
              description="请在设置页填写 OpenAI 兼容供应商的接口地址、模型与 API Key 后开始对话"
              cta-text="前往设置配置 AI 供应商"
              @cta="gotoSettings"
            />
            <div class="ai-status-ops">
              <u-button @click="retry">我已配置，刷新</u-button>
            </div>
          </template>
        </u-card-content>
      </u-card>
    </div>

    <u-ai-chat
      v-else
      :transport="transport"
      :tools="tools"
      :models="modelOptions"
      v-model:model="model"
      :system-prompt="systemPrompt"
      :max-tool-rounds="12"
      class="ai-chat"
    />
  </div>
</template>

<style scoped>
.ai-page {
  height: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

/* 组件根节点 height: 100%，父级给明确高度 */
.ai-chat {
  flex: 1;
  min-height: 0;
  height: 100%;
}

/* 状态卡在整页居中，不再左上角留半屏空白 */
.ai-status-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.ai-status-card {
  width: 100%;
  max-width: 560px;
}

.ai-status-ops {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-top: 16px;
}
</style>
