<script setup>
import { ref } from 'vue'
import MarkdownContent from './MarkdownContent.vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  rows: { type: Number, default: 7 },
  placeholder: { type: String, default: '' },
  maxlength: { type: Number, default: 20000 },
})
const emit = defineEmits(['update:modelValue'])
const mode = ref('edit')
const textarea = ref(null)
function focus() {
  if (mode.value !== 'edit') mode.value = 'edit'
  requestAnimationFrame(() => textarea.value?.focus())
}
defineExpose({ focus })

function wrap(prefix, suffix = prefix) {
  const el = textarea.value
  if (!el) return
  const start = el.selectionStart
  const end = el.selectionEnd
  const sel = props.modelValue.slice(start, end) || '文字'
  const next = props.modelValue.slice(0, start) + prefix + sel + suffix + props.modelValue.slice(end)
  emit('update:modelValue', next)
  requestAnimationFrame(() => {
    el.focus()
    el.setSelectionRange(start + prefix.length, start + prefix.length + sel.length)
  })
}
</script>

<template>
  <div class="editor-shell">
    <div class="editor-head">
      <strong>内容</strong>
      <div class="editor-tabs">
        <button type="button" class="editor-tab" :class="{ active: mode === 'edit' }" @click="mode = 'edit'">编辑</button>
        <button type="button" class="editor-tab" :class="{ active: mode === 'preview' }" @click="mode = 'preview'">预览</button>
      </div>
    </div>
    <div class="editor-toolbar" v-if="mode === 'edit'">
      <button type="button" @click="wrap('**')">B</button>
      <button type="button" @click="wrap('`')">代码</button>
      <button type="button" @click="wrap('- ', '')">列表</button>
      <button type="button" @click="wrap('$')">公式</button>
      <span>支持 Markdown 与 LaTeX，行间公式使用 $$...$$</span>
    </div>
    <textarea
      v-if="mode === 'edit'"
      ref="textarea"
      :value="modelValue"
      :rows="rows"
      :maxlength="maxlength"
      :placeholder="placeholder"
      @input="emit('update:modelValue', $event.target.value)"
    ></textarea>
    <div v-else class="editor-preview">
      <MarkdownContent :source="modelValue" />
    </div>
  </div>
</template>
