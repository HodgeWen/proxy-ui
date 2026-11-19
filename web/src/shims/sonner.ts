import type { ReactNode } from "react"
import { toast as heroToast } from "@heroui/react"

type ToastOptions = {
  duration?: number
}

type ToastFn = ((message: ReactNode, options?: ToastOptions) => string) & {
  success: (message: ReactNode, options?: ToastOptions) => string
  error: (message: ReactNode, options?: ToastOptions) => string
  info: (message: ReactNode, options?: ToastOptions) => string
  warning: (message: ReactNode, options?: ToastOptions) => string
}

export const toast: ToastFn = Object.assign(
  (message: ReactNode, options?: ToastOptions) => heroToast(message, options as never),
  {
    success: (message: ReactNode, options?: ToastOptions) =>
      heroToast.success(message, options as never),
    error: (message: ReactNode, options?: ToastOptions) =>
      heroToast.danger(message, options as never),
    info: (message: ReactNode, options?: ToastOptions) =>
      heroToast.info(message, options as never),
    warning: (message: ReactNode, options?: ToastOptions) =>
      heroToast.warning(message, options as never),
  }
)
