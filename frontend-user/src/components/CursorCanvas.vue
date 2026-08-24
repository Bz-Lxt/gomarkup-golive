<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { link } from '../lib/transport'
import { useConsole } from '../stores/console'

const store = useConsole()
const canvas = ref<HTMLCanvasElement | null>(null)
const local: { x: number; y: number }[] = []
let raf = 0

function draw() {
  const el = canvas.value
  if (!el) return
  const ctx = el.getContext('2d')
  if (!ctx) return
  const w = el.clientWidth
  const h = el.clientHeight
  if (el.width !== w || el.height !== h) {
    el.width = w
    el.height = h
  }
  ctx.fillStyle = '#0b121a'
  ctx.fillRect(0, 0, w, h)
  const stroke = (pts: { x: number; y: number }[], color: string) => {
    if (pts.length < 2) return
    ctx.beginPath()
    ctx.strokeStyle = color
    ctx.lineWidth = 2
    pts.forEach((p, i) => (i === 0 ? ctx.moveTo(p.x * w, p.y * h) : ctx.lineTo(p.x * w, p.y * h)))
    ctx.stroke()
  }
  stroke(local.slice(-200), '#3ee0c5')
  stroke(store.cursorRemote.slice(-200), '#f0b429')
  raf = requestAnimationFrame(draw)
}

function onMove(ev: PointerEvent) {
  const el = canvas.value
  if (!el || !store.connected) return
  const r = el.getBoundingClientRect()
  const x = (ev.clientX - r.left) / r.width
  const y = (ev.clientY - r.top) / r.height
  local.push({ x, y })
  if (local.length > 400) local.splice(0, 80)
  void link.sendCursor(x, y)
}

function clear() {
  store.ask('清空画布', '本地与回环轨迹都会被抹掉。', '清空', () => {
    local.length = 0
    store.cursorRemote = []
  })
}

onMounted(() => {
  raf = requestAnimationFrame(draw)
})
onUnmounted(() => cancelAnimationFrame(raf))
</script>

<template>
  <section class="panel flex min-h-[220px] flex-col p-3">
    <header class="mb-2 flex items-center justify-between">
      <h3 class="text-xs uppercase tracking-[0.2em] text-mute">鼠标轨迹 · 青=本地 琥珀=回环</h3>
      <button type="button" class="font-mono text-xs text-amber hover:text-ink" @click="clear">清空</button>
    </header>
    <canvas ref="canvas" class="min-h-[180px] w-full flex-1 cursor-crosshair rounded bg-panel2" @pointermove="onMove" />
  </section>
</template>
