<script setup lang="ts">
import { onUnmounted, ref } from 'vue'
import { link } from '../lib/transport'
import { useConsole } from '../stores/console'

const store = useConsole()
const video = ref<HTMLVideoElement | null>(null)
const canvas = ref<HTMLCanvasElement | null>(null)
const mode = ref<'synth' | 'camera'>('synth')
let timer: number | null = null
let osc: OscillatorNode | null = null
let ctx: AudioContext | null = null

function synthFrame() {
  const el = canvas.value
  if (!el) return
  const c = el.getContext('2d')
  if (!c) return
  const t = Date.now()
  c.fillStyle = `hsl(${(t / 20) % 360} 40% 18%)`
  c.fillRect(0, 0, el.width, el.height)
  c.fillStyle = '#3ee0c5'
  c.fillRect((t / 8) % el.width, 20, 40, 40)
  c.font = '14px Share Tech Mono'
  c.fillText(`FRAME ${t}`, 12, 18)
  el.toBlob((b) => {
    if (!b) return
    void b.arrayBuffer().then((buf) => link.sendVideo(new Uint8Array(buf)))
  }, 'image/jpeg', 0.6)
}

function startSynth() {
  stop()
  mode.value = 'synth'
  timer = window.setInterval(synthFrame, 100)
  ctx = new AudioContext()
  osc = ctx.createOscillator()
  const gain = ctx.createGain()
  gain.gain.value = 0.05
  osc.frequency.value = 220
  const proc = ctx.createScriptProcessor(1024, 1, 1)
  proc.onaudioprocess = (ev) => {
    const ch = ev.inputBuffer.getChannelData(0)
    const pcm = new Uint8Array(ch.length)
    for (let i = 0; i < ch.length; i++) pcm[i] = Math.max(0, Math.min(255, (ch[i] + 1) * 127))
    void link.sendAudio(pcm)
  }
  osc.connect(gain)
  gain.connect(proc)
  proc.connect(ctx.destination)
  osc.start()
}

async function startCam() {
  stop()
  mode.value = 'camera'
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true })
    if (video.value) video.value.srcObject = stream
    timer = window.setInterval(() => {
      const el = canvas.value
      const v = video.value
      if (!el || !v) return
      const c = el.getContext('2d')
      if (!c) return
      c.drawImage(v, 0, 0, el.width, el.height)
      el.toBlob((b) => {
        if (!b) return
        void b.arrayBuffer().then((buf) => link.sendVideo(new Uint8Array(buf)))
      }, 'image/jpeg', 0.5)
    }, 120)
  } catch (e) {
    store.toast('error', '无法打开摄像头，已回退合成源')
    startSynth()
  }
}

function stop() {
  if (timer) clearInterval(timer)
  timer = null
  osc?.stop()
  osc = null
  void ctx?.close()
  ctx = null
}

onUnmounted(stop)
defineExpose({ startSynth })
</script>

<template>
  <section class="panel p-3">
    <header class="mb-2 flex items-center justify-between">
      <h3 class="text-xs uppercase tracking-[0.2em] text-mute">媒体源</h3>
      <div class="flex gap-2">
        <button type="button" class="rounded border border-line px-2 py-1 font-mono text-xs" :class="mode==='synth' ? 'text-cyan border-cyan' : 'text-mute'" @click="startSynth">合成</button>
        <button type="button" class="rounded border border-line px-2 py-1 font-mono text-xs" :class="mode==='camera' ? 'text-cyan border-cyan' : 'text-mute'" @click="startCam">摄像头</button>
      </div>
    </header>
    <div class="relative">
      <video ref="video" class="hidden" autoplay muted playsinline />
      <canvas ref="canvas" width="320" height="180" class="w-full rounded bg-panel2" />
      <div class="absolute bottom-2 left-2 font-mono text-xs text-phosphor">fps {{ store.videoFps.toFixed(0) }}</div>
    </div>
    <div class="mt-3">
      <div class="mb-1 text-xs text-mute">音频电平 · 丢帧 {{ store.audioDrops }}</div>
      <div class="h-2 overflow-hidden rounded bg-line">
        <div class="h-full bg-phosphor" :style="{ width: Math.round(store.audioLevel * 100) + '%' }" />
      </div>
    </div>
  </section>
</template>
