import { reactive, readonly } from 'vue'

export type ToastKind = 'success' | 'error' | 'info'

export type ToastItem = {
  id: number
  kind: ToastKind
  title?: string
  message: string
  duration: number
}

let seq = 1
const state = reactive({ items: [] as ToastItem[] })

export const toasts = readonly(state)

export function toast(message: string, kind: ToastKind = 'info', duration = 3200, title?: string) {
  const id = seq++
  state.items.push({ id, kind, message, duration, title })
  if (duration > 0) {
    window.setTimeout(() => dismissToast(id), duration)
  }
  return id
}

export function toastSuccess(message: string, title = '成功') {
  return toast(message, 'success', 2800, title)
}

export function toastError(message: string, title = '错误') {
  return toast(message, 'error', 4500, title)
}

export function toastInfo(message: string, title?: string) {
  return toast(message, 'info', 3200, title)
}

export function dismissToast(id: number) {
  const i = state.items.findIndex((t) => t.id === id)
  if (i >= 0) state.items.splice(i, 1)
}
