<script setup lang="ts">
import { ref } from 'vue'
import { link } from '../lib/transport'
import { useConsole } from '../stores/console'

const store = useConsole()
const input = ref<HTMLInputElement | null>(null)

async function onChange() {
  const f = input.value?.files?.[0]
  if (!f) return
  store.fileProgress = 0
  store.fileAck = ''
  try {
    await link.sendFile(f)
  } catch (e) {
    store.toast('error', String(e))
  }
}
</script>

<template>
  <section class="panel p-3">
    <h3 class="text-xs uppercase tracking-[0.2em] text-mute">可靠通道 · 文件</h3>
    <p class="mt-2 font-mono text-xs text-mute">走 Bidi Stream，30% 丢包下 SHA-256 必须一致</p>
    <label class="mt-3 flex cursor-pointer items-center justify-between rounded-lg border border-dashed border-cyan/40 bg-panel2 px-3 py-3">
      <span class="text-sm text-cyan">选择文件上传</span>
      <input ref="input" type="file" class="hidden" @change="onChange" />
    </label>
    <div class="mt-3 h-1.5 overflow-hidden rounded bg-line">
      <div class="h-full bg-cyan" :style="{ width: store.fileProgress + '%' }" />
    </div>
    <p class="mt-2 font-mono text-xs text-phosphor">{{ store.fileAck }}</p>
  </section>
</template>
