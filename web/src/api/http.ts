// 统一请求封装：后端 envelope 解包、401 统一跳登录、错误统一 toast。
import { message } from '@veltra/desktop'

/** 后端统一响应结构 */
export interface Envelope<T> {
  success: boolean
  msg: string
  data: T
}

/** 请求错误：携带 HTTP 状态码与后端 msg */
export class ApiError extends Error {
  status: number

  constructor(status: number, msg: string) {
    super(msg)
    this.status = status
  }
}

/** 请求选项：body 自动 JSON 序列化，silent 关闭统一错误提示与 401 跳转 */
export interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown
  silent?: boolean
}

/** 401 回调：由 router 注册，避免请求层反向依赖路由 */
let onUnauthorized: (() => void) | null = null

/** 注册会话过期回调 */
export function setUnauthorizedHandler(handler: () => void): void {
  onUnauthorized = handler
}

/** 发起请求并解包 envelope，失败时统一 toast（silent 除外）并抛出 ApiError */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { body, silent, headers, ...rest } = options
  let resp: Response
  try {
    resp = await fetch(path, {
      ...rest,
      headers: {
        ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
        ...headers,
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    if (!silent) message.error('网络错误，请检查面板服务是否可达')
    throw new ApiError(0, '网络错误')
  }

  let envelope: Envelope<T>
  try {
    envelope = (await resp.json()) as Envelope<T>
  } catch {
    if (!silent) message.error(`请求失败（HTTP ${resp.status}）`)
    throw new ApiError(resp.status, `HTTP ${resp.status}`)
  }

  if (!resp.ok || !envelope.success) {
    if (!silent) message.error(envelope.msg)
    // 非静默请求的 401 视为会话过期，统一跳登录
    if (resp.status === 401 && !silent) onUnauthorized?.()
    throw new ApiError(resp.status, envelope.msg)
  }
  // 操作已生效但附带有警告（如数据已保存、内核热应用失败已回滚），透出给用户
  if (!silent && envelope.msg && envelope.msg !== 'ok') message.warn(envelope.msg)
  return envelope.data
}
