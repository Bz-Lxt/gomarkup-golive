<script setup lang="ts">
import { useConsole } from '../stores/console'
const store = useConsole()
function ok() {
  store.modal?.onOk()
  store.closeModal()
}
</script>

<template>
  <div v-if="store.modal" class="fixed inset-0 z-40 flex items-center justify-center bg-black/60 p-4">
    <div class="w-full max-w-md rounded-2xl border border-line bg-panel p-5 shadow-glow">
      <h3 class="font-display text-lg text-cyan">{{ store.modal.title }}</h3>
      <p class="mt-2 text-sm text-mute">{{ store.modal.body }}</p>
      <div class="mt-5 flex justify-end gap-3">
        <button type="button" class="rounded-md border border-line px-3 py-1.5 text-sm text-mute hover:text-ink" @click="store.closeModal()">取消</button>
        <button type="button" class="rounded-md bg-cyan px-3 py-1.5 text-sm text-bg" @click="ok">{{ store.modal.confirm }}</button>
      </div>
    </div>
  </div>
</template>
