<script setup lang="ts">
import { EditorState } from '@codemirror/state'
import { markdown } from '@codemirror/lang-markdown'
import { EditorView, keymap } from '@codemirror/view'
import { defaultKeymap } from '@codemirror/commands'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps<{ modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const host = ref<HTMLElement>()
let view: EditorView | undefined
let syncingFromParent = false

onMounted(() => {
  if (!host.value) return
  view = new EditorView({
    parent: host.value,
    state: EditorState.create({
      doc: props.modelValue,
      extensions: [
        markdown(),
        keymap.of(defaultKeymap),
        EditorView.lineWrapping,
        EditorView.updateListener.of((update) => {
          if (update.docChanged && !syncingFromParent) emit('update:modelValue', update.state.doc.toString())
        }),
      ],
    }),
  })
})

watch(() => props.modelValue, (value) => {
  if (!view || value === view.state.doc.toString()) return
  syncingFromParent = true
  try {
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } })
  } finally {
    syncingFromParent = false
  }
})

onBeforeUnmount(() => view?.destroy())
</script>

<template>
  <div ref="host" class="markdown-editor" data-testid="markdown-editor" aria-label="Markdown editor" />
</template>

<style scoped>
.markdown-editor { min-height: 360px; border: 1px solid #cbd3cd; background: #fff; font: 14px/1.6 ui-monospace, SFMono-Regular, Menlo, monospace; }
.markdown-editor :deep(.cm-editor) { min-height: 360px; outline: none; }
.markdown-editor :deep(.cm-scroller) { overflow: auto; }
.markdown-editor :deep(.cm-content) { padding: 14px; }
.markdown-editor :deep(.cm-focused) { outline: 2px solid #75a892; outline-offset: -2px; }
</style>
