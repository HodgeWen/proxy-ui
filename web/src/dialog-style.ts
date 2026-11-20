/** 弹框统一布局约定：全站 UDialog 的宽度样式只经此生成。
 *  - 固定宽度 + max-width 兜底，窄窗口自动收窄不溢出；近全屏弹框（如 AI 对话）用 94vw 上限
 *  - 高度一律不自管：UDialog 自带 90vh 内容滚动容器，内容超高时只有 body 滚动，header / footer 常驻
 *  - 需要铺满内容区的对话类弹框，用 #default="{ maximized }" 作用域参数给内容 100% 高度
 *  - 不要在内容外包 max-height / overflow 滚动容器：会与弹框自带滚动叠加成双滚动条 */
export function dialogStyle(px: number, maxVw = 96): string {
  return `width: ${px}px; max-width: ${maxVw}vw`
}
