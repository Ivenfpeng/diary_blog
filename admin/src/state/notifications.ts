import { ref } from 'vue'

export type NotificationType = 'success' | 'error' | 'info'

export interface OperationNotification {
  id: number
  type: NotificationType
  title: string
  message?: string
}

let nextNotificationID = 1

export const notifications = ref<OperationNotification[]>([])

export function notify(type: NotificationType, title: string, message?: string): OperationNotification {
  const notification = { id: nextNotificationID++, type, title, message }
  notifications.value = [...notifications.value, notification].slice(-5)
  return notification
}

export function notifySuccess(title: string, message?: string): OperationNotification {
  return notify('success', title, message)
}

export function notifyError(title: string, message?: string): OperationNotification {
  return notify('error', title, message)
}

export function notifyInfo(title: string, message?: string): OperationNotification {
  return notify('info', title, message)
}

export function dismissNotification(id: number): void {
  notifications.value = notifications.value.filter((notification) => notification.id !== id)
}

export function clearNotifications(): void {
  notifications.value = []
}
