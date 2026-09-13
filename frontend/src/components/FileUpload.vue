<script setup>
import { ref } from 'vue'
import api from '../api'

const props = defineProps({
  modelValue: { type: Array, default: () => [] }, // [{name, url}]
})
const emit = defineEmits(['update:modelValue'])
const hint = ref('')
const uploading = ref(false)
const fileInput = ref(null)

async function onPick(e) {
  const files = [...e.target.files]
  e.target.value = ''
  if (!files.length) return
  if (props.modelValue.length + files.length > 5) {
    hint.value = '最多保留 5 个文件'
    return
  }
  for (const f of files) {
    if (f.size > 10 * 1024 * 1024) {
      hint.value = `文件 ${f.name} 超过 10MB`
      return
    }
  }
  uploading.value = true
  hint.value = ''
  try {
    const { data } = await api.upload(files)
    emit('update:modelValue', [...props.modelValue, ...data.files])
  } catch (err) {
    hint.value = err.message
  } finally {
    uploading.value = false
  }
}

// 只从当前列表移除，不删除服务端文件：同一文件可能已被其它帖子引用
function remove(item) {
  emit('update:modelValue', props.modelValue.filter((f) => f.url !== item.url))
}
</script>

<template>
  <div class="file-upload">
    <input ref="fileInput" type="file" multiple class="hidden-input" @change="onPick" />
    <button type="button" class="filter-btn" :disabled="uploading" @click="fileInput.click()">
      {{ uploading ? '上传中…' : '选择文件' }}
    </button>
    <div v-if="modelValue.length" class="file-list">
      <span v-for="(f, i) in modelValue" :key="i" class="file-chip">
        <a :href="f.url" target="_blank" rel="noopener">{{ f.name }}</a>
        <button type="button" class="file-remove" @click="remove(f)">×</button>
      </span>
    </div>
    <small class="upload-help" v-if="hint">{{ hint }}</small>
  </div>
</template>

<style scoped>
.hidden-input {
  display: none;
}
.file-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}
.file-chip {
  background: #eef4f4;
  border-radius: 5px;
  padding: 4px 10px;
  font-size: 12px;
  color: var(--teal);
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.file-remove {
  border: 0;
  background: transparent;
  color: var(--muted);
  font-size: 15px;
  padding: 0;
  line-height: 1;
}
</style>
