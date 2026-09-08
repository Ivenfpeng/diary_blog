<script setup lang="ts">
import { Compartment, EditorState } from '@codemirror/state'
import { markdown } from '@codemirror/lang-markdown'
import { EditorView, keymap } from '@codemirror/view'
import { defaultKeymap } from '@codemirror/commands'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = withDefaults(defineProps<{ modelValue: string; disabled?: boolean }>(), { disabled: false })
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const host = ref<HTMLElement>()
let view: EditorView | undefined
let syncingFromParent = false
const editable = new Compartment()

function focusEditor(): void {
  if (props.disabled) return
  view?.focus()
}

onMounted(() => {
  if (!host.value) return
  view = new EditorView({
    parent: host.value,
    state: EditorState.create({
      doc: props.modelValue,
      extensions: [
        markdown(),
        keymap.of(defaultKeymap),
        editable.of(EditorView.editable.of(!props.disabled)),
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

watch(() => props.disabled, (disabled) => {
  view?.dispatch({ effects: editable.reconfigure(EditorView.editable.of(!disabled)) })
})

onBeforeUnmount(() => view?.destroy())
</script>

<template>
  <div ref="host" class="markdown-editor" data-testid="markdown-editor" aria-label="Markdown editor" @click.self="focusEditor" />
</template>

<style scoped>
.markdown-editor { min-height: 360px; border: 1px solid #cbd3cd; background: #fff; font: 14px/1.6 ui-monospace, SFMono-Regular, Menlo, monospace; cursor: text; }
.markdown-editor :deep(.cm-editor) { min-height: 360px; outline: none; }
.markdown-editor :deep(.cm-scroller) { min-height: 360px; overflow: auto; }
.markdown-editor :deep(.cm-content) { min-height: 360px; padding: 14px; box-sizing: border-box; cursor: text; }
.markdown-editor :deep(.cm-focused) { outline: 2px solid #75a892; outline-offset: -2px; }
</style>
