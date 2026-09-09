<script setup lang="ts">
import { AlertCircle, CheckCircle2, Info, X } from '@lucide/vue'
import { computed } from 'vue'
import { dismissNotification, notifications, type OperationNotification } from '../state/notifications'

const iconFor = computed(() => (notification: OperationNotification) => {
  if (notification.type === 'success') return CheckCircle2
  if (notification.type === 'error') return AlertCircle
  return Info
})
</script>

<template>
  <section v-if="notifications.length" class="notification-tray" aria-label="Operation notifications" aria-live="polite">
    <article
      v-for="notification in notifications"
      :key="notification.id"
      class="notification-card"
      :class="notification.type"
      :role="notification.type === 'error' ? 'alert' : 'status'"
    >
      <component :is="iconFor(notification)" :size="18" aria-hidden="true" />
      <div class="notification-copy">
        <strong>{{ notification.title }}</strong>
        <p v-if="notification.message">{{ notification.message }}</p>
      </div>
      <button type="button" class="notification-dismiss" aria-label="Dismiss notification" @click="dismissNotification(notification.id)">
        <X :size="15" aria-hidden="true" />
      </button>
    </article>
  </section>
</template>

<style scoped>
.notification-tray {
  position: fixed;
  top: 82px;
  right: clamp(14px, 3vw, 42px);
  z-index: 12;
  display: grid;
  width: min(420px, calc(100vw - 28px));
  gap: 10px;
  pointer-events: none;
}

.notification-card {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 12px;
  align-items: start;
  padding: 13px 14px;
  border: 1px solid rgb(188 204 194 / 86%);
  border-left-width: 5px;
  border-radius: 18px;
  background:
    linear-gradient(135deg, rgb(255 255 252 / 96%), rgb(245 249 244 / 94%));
  box-shadow: 0 18px 50px rgb(18 37 28 / 16%);
  color: var(--admin-ink);
  pointer-events: none;
  backdrop-filter: blur(18px);
}

.notification-card.success {
  border-left-color: #2f7659;
}

.notification-card.error {
  border-left-color: #b74242;
}

.notification-card.info {
  border-left-color: #9f6818;
}

.notification-copy {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.notification-copy strong {
  font-size: 13px;
  letter-spacing: -.01em;
}

.notification-copy p {
  margin: 0;
  color: var(--admin-muted);
  font-size: 12px;
  line-height: 1.45;
}

.notification-dismiss {
  display: inline-grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: #596961;
  cursor: pointer;
  pointer-events: auto;
}

.notification-dismiss:hover {
  border-color: #d4ddd7;
  background: rgb(255 255 255 / 74%);
}

@media (max-width: 720px) {
  .notification-tray {
    top: 76px;
    right: 10px;
    left: 10px;
    width: auto;
  }
}
</style>
