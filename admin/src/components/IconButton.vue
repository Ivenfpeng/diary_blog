<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{
  label: string
  icon: Component
}>()
</script>

<template>
  <button class="icon-button" type="button" :aria-label="label" :title="label">
    <component :is="icon" :size="18" :stroke-width="1.8" aria-hidden="true" />
  </button>
</template>
