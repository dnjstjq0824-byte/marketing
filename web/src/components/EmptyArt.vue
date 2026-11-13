<!-- Spot illustration for empty states. Line art in pine/gold, no external assets. -->
<template>
  <div class="empty-art" role="img" :aria-label="title">
    <svg v-if="art === 'plant'" viewBox="0 0 96 96" width="84" height="84" aria-hidden="true">
      <circle cx="72" cy="24" r="9" fill="none" stroke="#b97f00" stroke-width="2.5" />
      <path d="M48 88V52M48 64c-8 0-14-6-16-14 8 0 14 6 16 14zm0-10c8 0 14-6 16-14-8 0-14 6-16 14z" fill="none" stroke="#0f2b34" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
      <path d="M34 88h28l-3-12H37z" fill="#f7e8c8" stroke="#0f2b34" stroke-width="2.5" stroke-linejoin="round" />
      <path d="M28 52h8M60 40h8" stroke="#0e7490" stroke-width="2.5" stroke-linecap="round" />
    </svg>
    <svg v-else-if="art === 'tray'" viewBox="0 0 96 96" width="84" height="84" aria-hidden="true">
      <path d="M14 58h68l-8 22H22z" fill="#fffdf8" stroke="#0f2b34" stroke-width="2.5" stroke-linejoin="round" />
      <path d="M34 58V38h28v20" fill="none" stroke="#0f2b34" stroke-width="2.5" stroke-linejoin="round" />
      <path d="M42 46l6 6 10-12" fill="none" stroke="#067647" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
      <circle cx="72" cy="28" r="6" fill="#f7e8c8" stroke="#b97f00" stroke-width="2.5" />
    </svg>
    <svg v-else viewBox="0 0 96 96" width="84" height="84" aria-hidden="true">
      <circle cx="42" cy="42" r="20" fill="#fffdf8" stroke="#0f2b34" stroke-width="2.5" />
      <path d="M57 57l16 16" stroke="#0f2b34" stroke-width="2.5" stroke-linecap="round" />
      <path d="M32 42h20M42 32v20" stroke="#b97f00" stroke-width="2.5" stroke-linecap="round" />
      <path d="M70 22h10M75 17v10" stroke="#0e7490" stroke-width="2.5" stroke-linecap="round" />
    </svg>
    <strong>{{ title }}</strong>
    <span v-if="hint">{{ hint }}</span>
  </div>
</template>

<script setup>
defineProps({
  art: { type: String, default: 'plant' },
  title: { type: String, required: true },
  hint: { type: String, default: '' },
})
</script>
