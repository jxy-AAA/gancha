<script setup>
import { computed } from 'vue'

const props = defineProps({
  page: { type: Number, default: 1 },
  total: { type: Number, default: 0 },
  pageSize: { type: Number, default: 10 },
})
const emit = defineEmits(['change'])
const pages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))
const list = computed(() => {
  const cur = props.page
  const all = []
  for (let i = 1; i <= pages.value; i++) {
    if (i === 1 || i === pages.value || Math.abs(i - cur) <= 2) all.push(i)
    else if (all[all.length - 1] !== '…') all.push('…')
  }
  return all
})
</script>

<template>
  <div v-if="pages > 1" class="pagination">
    <button :disabled="page <= 1" @click="emit('change', page - 1)">上一页</button>
    <button
      v-for="(p, i) in list"
      :key="i"
      :class="{ active: p === page }"
      :disabled="p === '…'"
      @click="emit('change', p)"
    >
      {{ p }}
    </button>
    <button :disabled="page >= pages" @click="emit('change', page + 1)">下一页</button>
  </div>
</template>
