<!-- 设置页：UTabs 容器 + 6 个平级页签组件（./settings/）。keep-alive 让各页签组件切走保活，
     保留 DNS 未保存编辑等页签内状态；页签自身的加载 / 轮询清理见各组件内生命周期钩子 -->
<script setup lang="ts">
import { UTabs } from '@veltra/desktop'
import type { TabItem } from '@veltra/desktop'
import { shallowRef } from 'vue'

import AccountTab from './settings/AccountTab.vue'
import AiTab from './settings/AiTab.vue'
import DnsTab from './settings/DnsTab.vue'
import KernelTab from './settings/KernelTab.vue'
import PanelUpdateTab from './settings/PanelUpdateTab.vue'
import ThemeTab from './settings/ThemeTab.vue'

const active = shallowRef('theme')
const tabs: TabItem[] = [
  { key: 'theme', name: '主题' },
  { key: 'account', name: '账号' },
  { key: 'kernel', name: '内核' },
  { key: 'update', name: '面板更新' },
  { key: 'ai', name: 'AI 供应商' },
  { key: 'dns', name: 'DNS' },
]
</script>

<template>
  <u-tabs v-model="active" :items="tabs" keep-alive>
    <template #theme>
      <ThemeTab />
    </template>

    <template v-slot:account>
      <AccountTab />
    </template>

    <template #kernel>
      <KernelTab />
    </template>

    <template #update>
      <PanelUpdateTab />
    </template>

    <!-- 插槽书写顺序与页签显示顺序无关（显示顺序由 tabs 数组决定）；放在 #ai 之前是为了规避 vue-tsc 对末尾插槽的类型推断问题 -->
    <template #dns>
      <DnsTab />
    </template>

    <template #ai>
      <AiTab />
    </template>
  </u-tabs>
</template>
