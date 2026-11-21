/** 全站确认弹窗收敛：替代各处 `await messageConfirm({...}).onClosed` + 判断 confirm 的样板。
 *  - danger：删除 / 覆盖类破坏性操作传 true（确认键转危险色），删除类确认一律传 true
 *  - confirmText：确认键文案，默认「确认」；动作明确的场景传「删除 / 应用」等动词 */
import { messageConfirm } from '@veltra/desktop'

export async function confirmAction(
  title: string,
  tip: string,
  danger = false,
  confirmText = '确认',
): Promise<boolean> {
  const action = await messageConfirm({
    title,
    message: tip,
    confirmButtonText: confirmText,
    confirmButtonType: danger ? 'danger' : 'primary',
    cancelButtonText: '取消',
  }).onClosed
  return action === 'confirm'
}
