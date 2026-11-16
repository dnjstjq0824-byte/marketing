<template>
  <img :src="url" :alt="alt" loading="lazy" />
</template>

<script setup>
import { ref, watch } from 'vue'
import { blobUrl } from '../stores/blobimg.js'

const props = defineProps({
  src: { type: String, default: '' },
  alt: { type: String, default: '' },
})

const url = ref('')

async function load(src) {
  url.value = ''
  if (!src) return
  try {
    url.value = await blobUrl(src)
  } catch {
    url.value = ''
  }
}

watch(() => props.src, (s) => load(s), { immediate: true })
</script>
