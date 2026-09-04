import { readonly, ref } from 'vue'
import { ApiError, apiRequest } from '../api/client'
import type { LoginResponse, SessionResponse } from '../api/types'

const username = ref<string | null>(null)
const restored = ref(false)
let restoreRequest: Promise<boolean> | null = null

export const session = {
  username: readonly(username),
  restored: readonly(restored),
}

export function setSession(value: LoginResponse | SessionResponse): void {
  username.value = value.username
}

export function clearSession(): void {
  username.value = null
}

export async function restoreSession(): Promise<boolean> {
  if (restoreRequest) return restoreRequest

  restoreRequest = apiRequest<SessionResponse>('/api/auth/session')
    .then((value) => {
      setSession(value)
      return true
    })
    .catch((error: unknown) => {
      if (error instanceof ApiError && error.status === 401) {
        clearSession()
        return false
      }
      clearSession()
      return false
    })
    .finally(() => {
      restored.value = true
    })

  return restoreRequest
}
