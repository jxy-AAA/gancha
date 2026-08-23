<script setup>
import { computed } from 'vue'
import { renderMarkdown } from '../utils/markdown'

const props = defineProps({
  source: { type: String, default: '' },
})
const html = computed(() => renderMarkdown(props.source))
</script>

<template>
  <div class="rich-content" v-html="html"></div>
</template>
