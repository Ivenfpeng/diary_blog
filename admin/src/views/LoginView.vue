<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiRequest } from '../api/client'
import type { LoginResponse } from '../api/types'
import { setSession } from '../state/session'

const router = useRouter()
const username = ref('')
const password = ref('')
const errorMessage = ref('')
const passwordInput = ref<HTMLInputElement>()
const submitting = ref(false)

async function submit(): Promise<void> {
  errorMessage.value = ''
  submitting.value = true
  try {
    const response = await apiRequest<LoginResponse>('/api/auth/login', {
      method: 'POST',
      body: { username: username.value, password: password.value },
    })
    setSession(response)
    await router.push({ name: 'dashboard' })
  } catch (error) {
    password.value = ''
    errorMessage.value = error instanceof Error ? error.message : 'Unable to sign in.'
    passwordInput.value?.focus()
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="login-page">
    <form class="login-form" @submit.prevent="submit">
      <p class="eyebrow">Diary Blog</p>
      <h1>Sign in</h1>
      <p class="login-intro">Use your administrator account to manage published work.</p>
      <p v-if="errorMessage" class="form-error" role="alert">{{ errorMessage }}</p>
      <label for="username">Username</label>
      <input id="username" v-model="username" name="username" autocomplete="username" required />
      <label for="password">Password</label>
      <input id="password" ref="passwordInput" v-model="password" name="password" type="password" autocomplete="current-password" required />
      <button class="primary-button" type="submit" :disabled="submitting">
        {{ submitting ? 'Signing in…' : 'Sign in' }}
      </button>
    </form>
  </main>
</template>
