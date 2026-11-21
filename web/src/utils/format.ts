/** 展示层通用格式化：字节数、运行时长与日期时间，各页面共用，勿在视图内再各写一份。 */
import { date } from '@cat-kit/core'

/** 字节数转人类可读（如 1.5 MB） */
export function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const value = bytes / 1024 ** i
  return `${value >= 100 || i === 0 ? Math.round(value) : value.toFixed(value >= 10 ? 1 : 2)} ${units[i]}`
}

/** 秒数转 HH:MM:SS 运行时长；未知时长显示 - */
export function formatUptime(seconds?: number): string {
  if (seconds === undefined) return '-'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  return [h, m, s].map((n) => String(n).padStart(2, '0')).join(':')
}

/** 格式化日期时间为 yyyy-MM-dd HH:mm:ss；无效时间显示 - */
export function formatDateTime(value: string): string {
  if (!value) return '-'
  const d = date(value)
  return Number.isNaN(d.timestamp) ? '-' : d.format('yyyy-MM-dd HH:mm:ss')
}
