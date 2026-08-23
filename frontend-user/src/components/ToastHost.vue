<script setup lang="ts">
import { useConsole } from '../stores/console'
const store = useConsole()
</script>

<template>
  <div class="fixed right-4 top-4 z-50 flex w-full max-w-sm flex-col gap-2 px-3 sm:px-0">
    <div
      v-for="t in store.toasts"
      :key="t.id"
      class="flex items-start gap-3 rounded-lg border px-3 py-2 shadow-glow"
      :class="t.kind === 'error' ? 'border-danger/50 bg-[#2a1118] text-danger' : t.kind === 'ok' ? 'border-phosphor/40 bg-[#102016] text-phosphor' : 'border-cyan/40 bg-[#102018] text-cyan'"
    >
      <p class="font-mono text-sm leading-5">{{ t.text }}</p>
      <button class="ml-auto text-mute hover:text-ink" type="button" @click="store.dismiss(t.id)">×</button>
    </div>
  </div>
</template>
