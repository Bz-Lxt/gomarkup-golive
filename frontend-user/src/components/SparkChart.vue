<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import type { Point } from '../stores/console'

const props = defineProps<{
  title: string
  aLabel: string
  bLabel: string
  points: Point[]
  aColor?: string
  bColor?: string
  unit?: string
}>()

const canvas = ref<HTMLCanvasElement | null>(null)
let raf = 0

function draw() {
  const el = canvas.value
  if (!el) return
  const dpr = window.devicePixelRatio || 1
  const w = el.clientWidth
  const h = el.clientHeight
  if (el.width !== w * dpr) {
    el.width = w * dpr
    el.height = h * dpr
  }
  const ctx = el.getContext('2d')
  if (!ctx) return
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, w, h)
  ctx.fillStyle = '#121c28'
  ctx.fillRect(0, 0, w, h)
  ctx.strokeStyle = '#1c2c3c'
  ctx.lineWidth = 1
  for (let i = 1; i < 4; i++) {
    ctx.beginPath()
    ctx.moveTo(0, (h * i) / 4)
    ctx.lineTo(w, (h * i) / 4)
    ctx.stroke()
  }
  const pts = props.points
  if (pts.length < 2) return
  const max = Math.max(1, ...pts.flatMap((p) => [p.a, p.b]))
  const plot = (key: 'a' | 'b', color: string) => {
    ctx.beginPath()
    ctx.strokeStyle = color
    ctx.lineWidth = 1.6
    pts.forEach((p, i) => {
      const x = (i / (pts.length - 1)) * w
      const y = h - (p[key] / max) * (h - 8) - 4
      if (i === 0) ctx.moveTo(x, y)
      else ctx.lineTo(x, y)
    })
    ctx.stroke()
  }
  plot('a', props.aColor || '#3ee0c5')
  plot('b', props.bColor || '#8cff6b')
  const last = pts[pts.length - 1]
  ctx.fillStyle = '#7b8b99'
  ctx.font = '12px "Share Tech Mono"'
  ctx.fillText(`${props.aLabel} ${last.a.toFixed(1)}  ${props.bLabel} ${last.b.toFixed(1)} ${props.unit || ''}`, 8, 16)
}

function loop() {
  draw()
  raf = requestAnimationFrame(loop)
}

onMounted(() => {
  raf = requestAnimationFrame(loop)
})
onUnmounted(() => cancelAnimationFrame(raf))
watch(() => props.points.length, draw)
</script>

<template>
  <section class="panel relative overflow-hidden p-3">
    <header class="mb-2 flex items-center justify-between">
      <h3 class="text-xs uppercase tracking-[0.2em] text-mute">{{ title }}</h3>
    </header>
    <canvas ref="canvas" class="h-40 w-full" />
  </section>
</template>
