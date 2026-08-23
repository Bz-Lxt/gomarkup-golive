<script setup lang="ts">
import { computed } from 'vue'
import { useConsole } from '../stores/console'

const store = useConsole()
const color: Record<string, string> = {
  Startup: '#f0b429',
  Drain: '#ff4d6d',
  ProbeBW: '#9b8cff',
  ProbeRTT: '#3ee0c5',
}
const last = computed(() => store.bbr[store.bbr.length - 1])
</script>

<template>
  <section class="panel p-3">
    <div class="mb-2 flex items-center justify-between">
      <h3 class="text-xs uppercase tracking-[0.2em] text-mute">BBR 状态时间线 · 应用层调度器</h3>
      <span class="font-mono text-xs text-amber">{{ last?.state || '—' }}</span>
    </div>
    <div class="flex h-6 w-full overflow-hidden rounded">
      <div
        v-for="(p, i) in store.bbr.slice(-80)"
        :key="i"
        class="h-full flex-1"
        :style="{ background: color[p.state] || '#1c2c3c' }"
        :title="p.state"
      />
    </div>
    <dl class="mt-3 grid grid-cols-3 gap-2 font-mono text-xs text-mute">
      <div>pacing {{ ((last?.pacing || 0) / 1000).toFixed(0) }} kbps</div>
      <div>BtlBw {{ ((last?.btl || 0) / 1000).toFixed(0) }} kbps</div>
      <div>gain {{ (last?.gain || 0).toFixed(2) }}</div>
    </dl>
  </section>
</template>
