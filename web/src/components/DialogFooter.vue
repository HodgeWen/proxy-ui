<!-- 弹窗 footer 的「取消 / 确认」按钮组合：只管按钮形态，取消与确认都抛事件，关弹窗由调用方决定 -->
<script setup lang="ts">
import { UButton } from '@veltra/desktop'

withDefaults(defineProps<{ loading?: boolean; confirmText?: string }>(), { confirmText: '保存' })
defineEmits<{ cancel: []; confirm: [] }>()
</script>

<template>
  <u-button text @click="$emit('cancel')">取消</u-button>
  <u-button type="primary" :loading="loading" @click="$emit('confirm')">{{ confirmText }}</u-button>
</template>
