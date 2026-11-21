/** 公共剪贴板复制：写入系统剪贴板并给出统一结果提示。 */
import { clipboard } from '@cat-kit/fe'
import { message } from '@veltra/desktop'

/** 复制文本到剪贴板并提示成功或失败 */
export async function copyText(text: string): Promise<void> {
  try {
    await clipboard.copy(text)
    message.success('已复制')
  } catch {
    message.error('复制失败，请手动复制')
  }
}
