<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Bold, Heading1, Heading2, ImagePlus, Italic, List, Pilcrow, Quote } from '@lucide/vue'

export interface UploadedEditorImage {
  src: string
  alt: string
}

const props = withDefaults(defineProps<{
  modelValue: string
  disabled?: boolean
  uploadImage?: (file: File) => Promise<UploadedEditorImage>
}>(), { disabled: false })
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const surface = ref<HTMLElement>()
const fileInput = ref<HTMLInputElement>()
const uploadError = ref('')
const uploading = ref(false)
let syncingFromParent = false

const editorLabel = computed(() => props.disabled ? 'Rich text editor disabled' : 'Rich text editor')

function escapeHTML(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
}

function escapeAttribute(value: string): string {
  return escapeHTML(value).replaceAll("'", '&#39;')
}

function inlineMarkdownToHTML(value: string): string {
  return escapeHTML(value)
    .replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img src="$2" alt="$1">')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\*([^*]+)\*/g, '<em>$1</em>')
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>')
}

function markdownToHTML(markdown: string): string {
  const blocks: string[] = []
  const lines = markdown.replace(/\r\n?/g, '\n').split('\n')
  let paragraph: string[] = []
  let list: string[] = []
  let quote: string[] = []
  let code: string[] | null = null

  const flushParagraph = () => {
    if (!paragraph.length) return
    blocks.push(`<p>${inlineMarkdownToHTML(paragraph.join(' '))}</p>`)
    paragraph = []
  }
  const flushList = () => {
    if (!list.length) return
    blocks.push(`<ul>${list.map((item) => `<li>${inlineMarkdownToHTML(item)}</li>`).join('')}</ul>`)
    list = []
  }
  const flushQuote = () => {
    if (!quote.length) return
    blocks.push(`<blockquote>${quote.map((item) => `<p>${inlineMarkdownToHTML(item)}</p>`).join('')}</blockquote>`)
    quote = []
  }
  const flushTextBlocks = () => {
    flushParagraph()
    flushList()
    flushQuote()
  }

  for (const line of lines) {
    if (line.startsWith('```')) {
      if (code) {
        blocks.push(`<pre><code>${escapeHTML(code.join('\n'))}</code></pre>`)
        code = null
      } else {
        flushTextBlocks()
        code = []
      }
      continue
    }
    if (code) {
      code.push(line)
      continue
    }
    if (!line.trim()) {
      flushTextBlocks()
      continue
    }
    const image = line.match(/^!\[([^\]]*)\]\(([^)]+)\)$/)
    if (image) {
      flushTextBlocks()
      blocks.push(`<figure><img src="${escapeAttribute(image[2])}" alt="${escapeAttribute(image[1])}"><figcaption>${escapeHTML(image[1])}</figcaption></figure>`)
      continue
    }
    const heading = line.match(/^(#{1,3})\s+(.+)$/)
    if (heading) {
      flushTextBlocks()
      blocks.push(`<h${heading[1].length}>${inlineMarkdownToHTML(heading[2])}</h${heading[1].length}>`)
      continue
    }
    const listItem = line.match(/^[-*]\s+(.+)$/)
    if (listItem) {
      flushParagraph()
      flushQuote()
      list.push(listItem[1])
      continue
    }
    const quoted = line.match(/^>\s?(.+)$/)
    if (quoted) {
      flushParagraph()
      flushList()
      quote.push(quoted[1])
      continue
    }
    flushList()
    flushQuote()
    paragraph.push(line)
  }

  if (code) blocks.push(`<pre><code>${escapeHTML(code.join('\n'))}</code></pre>`)
  flushTextBlocks()
  return blocks.join('') || '<p><br></p>'
}

function normalizeWhitespace(value: string): string {
  return value.replace(/\u00a0/g, ' ').replace(/[ \t]+\n/g, '\n').trim()
}

function inlineHTMLToMarkdown(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) return node.textContent ?? ''
  if (!(node instanceof HTMLElement)) return ''

  const content = Array.from(node.childNodes).map(inlineHTMLToMarkdown).join('')
  const tag = node.tagName.toLowerCase()
  if (tag === 'strong' || tag === 'b') return `**${content}**`
  if (tag === 'em' || tag === 'i') return `*${content}*`
  if (tag === 'a') return `[${content}](${node.getAttribute('href') ?? ''})`
  if (tag === 'br') return '\n'
  if (tag === 'img') return `![${node.getAttribute('alt') ?? ''}](${node.getAttribute('src') ?? ''})`
  return content
}

function blockToMarkdown(element: Element): string {
  const tag = element.tagName.toLowerCase()
  if (tag === 'h1') return `# ${normalizeWhitespace(inlineHTMLToMarkdown(element))}`
  if (tag === 'h2') return `## ${normalizeWhitespace(inlineHTMLToMarkdown(element))}`
  if (tag === 'h3') return `### ${normalizeWhitespace(inlineHTMLToMarkdown(element))}`
  if (tag === 'blockquote') {
    return Array.from(element.children)
      .map((child) => `> ${normalizeWhitespace(inlineHTMLToMarkdown(child))}`)
      .join('\n')
  }
  if (tag === 'ul') {
    return Array.from(element.children)
      .filter((child) => child.tagName.toLowerCase() === 'li')
      .map((child) => `- ${normalizeWhitespace(inlineHTMLToMarkdown(child))}`)
      .join('\n')
  }
  if (tag === 'ol') {
    return Array.from(element.children)
      .filter((child) => child.tagName.toLowerCase() === 'li')
      .map((child, index) => `${index + 1}. ${normalizeWhitespace(inlineHTMLToMarkdown(child))}`)
      .join('\n')
  }
  if (tag === 'pre') return `\`\`\`\n${element.textContent?.trimEnd() ?? ''}\n\`\`\``
  if (tag === 'figure') {
    const image = element.querySelector('img')
    if (image) return `![${image.getAttribute('alt') ?? ''}](${image.getAttribute('src') ?? ''})`
  }
  if (tag === 'img') return `![${element.getAttribute('alt') ?? ''}](${element.getAttribute('src') ?? ''})`
  return normalizeWhitespace(inlineHTMLToMarkdown(element))
}

function htmlToMarkdown(element: HTMLElement): string {
  const markdown = Array.from(element.childNodes)
    .map((node) => {
      if (node.nodeType === Node.TEXT_NODE) return normalizeWhitespace(node.textContent ?? '')
      if (node instanceof Element) return blockToMarkdown(node)
      return ''
    })
    .filter(Boolean)
    .join('\n\n')
  return markdown || normalizeWhitespace(element.textContent ?? '')
}

function syncSurfaceFromMarkdown(markdown: string): void {
  if (!surface.value) return
  syncingFromParent = true
  surface.value.innerHTML = markdownToHTML(markdown)
  syncingFromParent = false
}

function emitCurrentMarkdown(): void {
  if (!surface.value || syncingFromParent) return
  emit('update:modelValue', htmlToMarkdown(surface.value))
}

function focusEditor(): void {
  if (props.disabled) return
  surface.value?.focus()
}

function placeCaretAtEnd(): void {
  if (!surface.value) return
  const range = document.createRange()
  range.selectNodeContents(surface.value)
  range.collapse(false)
  const selection = window.getSelection()
  selection?.removeAllRanges()
  selection?.addRange(range)
  surface.value.focus()
}

function focusEditorFromPointer(event: MouseEvent): void {
  if (props.disabled || !surface.value) return
  const target = event.target
  if (!(target instanceof HTMLElement)) return
  if (target.closest('.rich-editor-toolbar, button, input')) return
  if (surface.value.contains(target) || target === event.currentTarget) {
    event.preventDefault()
    placeCaretAtEnd()
    window.requestAnimationFrame(() => placeCaretAtEnd())
  }
}

function runCommand(command: string, value?: string): void {
  if (props.disabled) return
  focusEditor()
  if (typeof document.execCommand === 'function') {
    document.execCommand(command, false, value)
  }
  emitCurrentMarkdown()
}

function setBlock(tag: 'p' | 'h1' | 'h2' | 'blockquote'): void {
  runCommand('formatBlock', tag)
}

function openImagePicker(): void {
  if (props.disabled || uploading.value) return
  uploadError.value = ''
  fileInput.value?.click()
}

function filenameAlt(file: File): string {
  return file.name.replace(/\.[^.]+$/, '').replace(/[-_]+/g, ' ').trim() || 'Uploaded image'
}

async function uploadSelectedImage(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!props.uploadImage) {
    uploadError.value = 'Image upload is not available in this editor.'
    input.value = ''
    return
  }
  uploadError.value = ''
  uploading.value = true
  try {
    const uploaded = await props.uploadImage(file)
    const alt = uploaded.alt || filenameAlt(file)
    const figure = `<figure><img src="${escapeAttribute(uploaded.src)}" alt="${escapeAttribute(alt)}"><figcaption>${escapeHTML(alt)}</figcaption></figure><p><br></p>`
    focusEditor()
    if (typeof document.execCommand === 'function') {
      document.execCommand('insertHTML', false, figure)
    }
    const inserted = surface.value
      ? Array.from(surface.value.querySelectorAll('img')).some((image) => image.getAttribute('src') === uploaded.src)
      : false
    if (surface.value && !inserted) {
      surface.value.insertAdjacentHTML('beforeend', figure)
    }
    emitCurrentMarkdown()
  } catch (error) {
    uploadError.value = error instanceof Error ? error.message : 'Image upload failed.'
  } finally {
    uploading.value = false
    input.value = ''
  }
}

watch(() => props.modelValue, (value) => {
  if (!surface.value || value === htmlToMarkdown(surface.value)) return
  syncSurfaceFromMarkdown(value)
})

onMounted(() => syncSurfaceFromMarkdown(props.modelValue))
</script>

<template>
  <div class="markdown-editor rich-editor" data-testid="markdown-editor" :aria-label="editorLabel" @mousedown="focusEditorFromPointer" @click.self="focusEditor">
    <div class="rich-editor-toolbar" aria-label="Rich text formatting toolbar">
      <button type="button" :disabled="disabled" aria-label="Paragraph" @click="setBlock('p')"><Pilcrow :size="16" aria-hidden="true" /> Text</button>
      <button type="button" :disabled="disabled" aria-label="Heading 1" @click="setBlock('h1')"><Heading1 :size="16" aria-hidden="true" /></button>
      <button type="button" :disabled="disabled" aria-label="Heading 2" @click="setBlock('h2')"><Heading2 :size="16" aria-hidden="true" /></button>
      <button type="button" :disabled="disabled" aria-label="Bold" @click="runCommand('bold')"><Bold :size="16" aria-hidden="true" /></button>
      <button type="button" :disabled="disabled" aria-label="Italic" @click="runCommand('italic')"><Italic :size="16" aria-hidden="true" /></button>
      <button type="button" :disabled="disabled" aria-label="Bullet list" @click="runCommand('insertUnorderedList')"><List :size="16" aria-hidden="true" /></button>
      <button type="button" :disabled="disabled" aria-label="Quote" @click="setBlock('blockquote')"><Quote :size="16" aria-hidden="true" /></button>
      <button type="button" :disabled="disabled || uploading" aria-label="Upload image" @click="openImagePicker"><ImagePlus :size="16" aria-hidden="true" /> {{ uploading ? 'Uploading…' : 'Image' }}</button>
      <input id="rich-editor-image-upload" ref="fileInput" class="image-input" type="file" accept="image/jpeg,image/png,image/webp,image/gif,image/avif" :disabled="disabled || uploading" @change="uploadSelectedImage" />
    </div>
    <div
      ref="surface"
      class="rich-editor-surface"
      role="textbox"
      tabindex="0"
      aria-multiline="true"
      data-language="rich-text"
      :aria-disabled="disabled"
      :contenteditable="!disabled"
      @input="emitCurrentMarkdown"
      @blur="emitCurrentMarkdown"
    />
    <p v-if="uploadError" class="rich-editor-error" role="alert">{{ uploadError }}</p>
  </div>
</template>

<style scoped>
.rich-editor { --markdown-editor-height: clamp(420px, 52vh, 760px); overflow: hidden; border: 1px solid #bdcac2; border-radius: 18px; background: #fffef8; box-shadow: inset 0 1px 0 rgb(255 255 255 / 80%), 0 16px 42px rgb(20 38 30 / 8%); cursor: text; }
.rich-editor-toolbar { position: sticky; top: 0; z-index: 1; display: flex; flex-wrap: wrap; gap: 8px; padding: 10px; border-bottom: 1px solid #dce6df; background: linear-gradient(180deg, #fffefb 0%, #f5faf5 100%); cursor: default; }
.rich-editor-toolbar button { min-height: 34px; display: inline-flex; align-items: center; gap: 6px; border: 1px solid #bfcdc4; border-radius: 11px; padding: 0 10px; background: #fff; color: #203d31; font: inherit; font-size: 13px; font-weight: 750; cursor: pointer; }
.rich-editor-toolbar button:hover:not(:disabled) { border-color: #7fa28f; background: #eff7f1; }
.rich-editor-toolbar button:disabled { cursor: not-allowed; opacity: .55; }
.image-input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; pointer-events: none; }
.rich-editor-surface { min-height: var(--markdown-editor-height); padding: 22px clamp(18px, 2vw, 30px); outline: none; color: #1e2d27; font: 16px/1.75 ui-serif, Georgia, Cambria, "Times New Roman", serif; }
.rich-editor-surface:focus { outline: 3px solid rgb(72 126 98 / 26%); outline-offset: -3px; }
.rich-editor-surface :deep(h1), .rich-editor-surface :deep(h2), .rich-editor-surface :deep(h3) { margin: .15em 0 .55em; color: #13251d; font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; line-height: 1.08; letter-spacing: -.035em; }
.rich-editor-surface :deep(h1) { font-size: clamp(28px, 3vw, 42px); }
.rich-editor-surface :deep(h2) { font-size: clamp(22px, 2.2vw, 32px); }
.rich-editor-surface :deep(p) { margin: 0 0 1em; }
.rich-editor-surface :deep(blockquote) { margin: 1.1em 0; padding: .8em 1em; border-left: 4px solid #6d987f; border-radius: 12px; background: #f2f7f1; color: #354940; }
.rich-editor-surface :deep(ul), .rich-editor-surface :deep(ol) { padding-left: 1.4em; }
.rich-editor-surface :deep(figure) { margin: 1.2em 0; padding: 12px; border: 1px solid #d6e1d9; border-radius: 18px; background: #f8fbf7; }
.rich-editor-surface :deep(img) { max-width: 100%; height: auto; display: block; border-radius: 14px; box-shadow: 0 14px 34px rgb(26 44 35 / 12%); }
.rich-editor-surface :deep(figcaption) { margin-top: 8px; color: #68756d; font: 13px/1.4 ui-sans-serif, system-ui, sans-serif; }
.rich-editor-surface :deep(pre) { overflow: auto; padding: 14px; border-radius: 14px; background: #12221a; color: #eff8f0; font: 13px/1.65 ui-monospace, SFMono-Regular, Menlo, monospace; }
.rich-editor-error { margin: 0; padding: 10px 14px; border-top: 1px solid #edc3bf; color: #8d3028; background: #fff2f1; font-size: 13px; }
@media (max-width: 720px) { .rich-editor { --markdown-editor-height: 340px; }.rich-editor-surface { padding: 18px; } }
</style>
