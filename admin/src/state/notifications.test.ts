import { afterEach, describe, expect, it, vi } from 'vitest'
import { clearNotifications, dismissNotification, notifications, notifyError, notifySuccess } from './notifications'

describe('notifications', () => {
  afterEach(() => {
    clearNotifications()
    vi.useRealTimers()
  })

  it('records success and failure messages until dismissed', () => {
    clearNotifications()

    const success = notifySuccess('Settings saved', 'Your public site metadata was updated.')
    const failure = notifyError('Save failed', 'Try again after checking the required fields.')

    expect(notifications.value).toMatchObject([
      { id: success.id, type: 'success', title: 'Settings saved', message: 'Your public site metadata was updated.' },
      { id: failure.id, type: 'error', title: 'Save failed', message: 'Try again after checking the required fields.' },
    ])

    dismissNotification(success.id)

    expect(notifications.value).toMatchObject([
      { id: failure.id, type: 'error', title: 'Save failed' },
    ])
  })

  it('automatically dismisses notifications after three seconds', () => {
    vi.useFakeTimers()
    clearNotifications()

    const notification = notifySuccess('Settings saved')
    expect(notifications.value).toHaveLength(1)

    vi.advanceTimersByTime(2999)
    expect(notifications.value).toHaveLength(1)

    vi.advanceTimersByTime(1)
    expect(notifications.value).toHaveLength(0)
    expect(notification.id).toBeGreaterThan(0)
  })
})
