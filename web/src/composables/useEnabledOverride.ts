/**
 * 启停覆写表：USwitch 内部是原生 checkbox，请求失败时父级数据不变、`:checked` 不会重新下刷，
 * 会和 DOM 状态错位（下一次点击发出的仍是旧值）。用本地覆写表驱动开关，
 * 失败时写回相反值即触发一次真实的下刷；父级回读出新数据后由 watch 清空覆写。
 */
import { shallowRef, toValue, watch } from 'vue'
import type { MaybeRefOrGetter } from 'vue'

/**
 * 行启停的共享逻辑：rowEnabled 供开关绑定，toggle 承接点击。
 * rows 传行列表（父级回读后引用变化即清空覆写）；toggleAction 里做真正的启停请求与后续动作，
 * 抛错时这里回滚覆写值让开关立即恢复（错误提示由请求封装统一负责）。
 */
export function useEnabledOverride<T extends { id: number; enabled: boolean }>(
  rows: MaybeRefOrGetter<T[]>,
  toggleAction: (row: T, enabled: boolean) => Promise<void>,
) {
  const enabledOverride = shallowRef<Record<number, boolean>>({})
  const togglingId = shallowRef(0)

  function rowEnabled(row: T): boolean {
    return enabledOverride.value[row.id] ?? row.enabled
  }

  // 父级回读出新数据后，覆写表已完成使命，清空以免展示过期状态
  watch(
    () => toValue(rows),
    () => {
      enabledOverride.value = {}
    },
  )

  async function toggle(row: T, enabled: boolean): Promise<void> {
    togglingId.value = row.id
    enabledOverride.value = { ...enabledOverride.value, [row.id]: enabled }
    try {
      await toggleAction(row, enabled)
    } catch {
      // 失败回滚：写回相反值让开关立即恢复
      enabledOverride.value = { ...enabledOverride.value, [row.id]: !enabled }
    } finally {
      togglingId.value = 0
    }
  }

  return { rowEnabled, togglingId, toggle }
}
