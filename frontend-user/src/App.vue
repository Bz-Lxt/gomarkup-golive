<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useConsole } from './stores/console'
import { link } from './lib/transport'
import ToastHost from './components/ToastHost.vue'
import ModalHost from './components/ModalHost.vue'
import SparkChart from './components/SparkChart.vue'
import BBRStrip from './components/BBRStrip.vue'
import CursorCanvas from './components/CursorCanvas.vue'
import FileLane from './components/FileLane.vue'
import MediaLane from './components/MediaLane.vue'
import Unsupported from './components/Unsupported.vue'

const store = useConsole()
const media = ref<{ startSynth: () => void } | null>(null)
let pingTimer: number | null = null

async function connect() {
  try {
    await link.connect()
    media.value?.startSynth()
    pingTimer = window.setInterval(() => void link.ping(), 1000)
  } catch (e) {
    store.connecting = false
    store.toast('error', String(e))
  }
}

function disconnect() {
  store.ask('断开 WebTransport', '当前会话、回环画布与媒体泵都会停下。', '断开', () => {
    if (pingTimer) clearInterval(pingTimer)
    void link.disconnect()
  })
}

async function changePreset(p: string) {
  try {
    await link.setPreset(p)
  } catch (e) {
    store.toast('error', String(e))
  }
}

onMounted(async () => {
  if (!store.supported) return
  try {
    const cfg = await (await fetch('/api/v1/config')).json()
    store.wtUrl = cfg.wt_url
    store.bbrLayer = cfg.bbr_layer
    const fp = await (await fetch('/api/v1/cert-fingerprint')).json()
    store.certHours = fp.valid_hours
  } catch {
    /* health later */
  }
})
onUnmounted(() => {
  if (pingTimer) clearInterval(pingTimer)
})
</script>

<template>
  <Unsupported v-if="!store.supported" />
  <div v-else class="min-h-screen w-full">
    <ToastHost />
    <ModalHost />
    <header class="flex w-full flex-wrap items-center gap-4 border-b border-line px-4 py-3 md:px-6">
      <div>
        <p class="font-mono text-[10px] uppercase tracking-[0.35em] text-cyan">QUIC / HTTP3 / WEBTRANSPORT</p>
        <h1 class="font-display text-2xl font-semibold">GoLive 弱网对抗台</h1>
      </div>
      <div class="ml-auto flex flex-wrap items-center gap-3 font-mono text-xs text-mute">
        <span>sid {{ store.sessionId.slice(0, 8) || '—' }}</span>
        <span>cert {{ store.certHours }}h</span>
        <span>GSO {{ store.gso ? 'on' : 'off' }}</span>
        <span>dgram {{ store.datagram ? 'yes' : 'no' }}</span>
        <span>QUIC {{ store.quicVersion || '—' }}</span>
        <button
          v-if="!store.connected"
          type="button"
          class="rounded-md bg-cyan px-3 py-1.5 text-bg disabled:opacity-40"
          :disabled="store.connecting"
          @click="connect"
        >{{ store.connecting ? '握手中…' : '连接' }}</button>
        <button v-else type="button" class="rounded-md border border-danger/50 px-3 py-1.5 text-danger" @click="disconnect">断开</button>
      </div>
    </header>

    <main class="w-full px-4 py-4 md:px-6">
      <div class="grid w-full grid-cols-1 gap-4 lg:grid-cols-12">
        <div class="flex flex-col gap-4 lg:col-span-8">
          <SparkChart title="RTT 双源" a-label="传输层" b-label="应用层" unit="ms" :points="store.rtt" />
          <SparkChart title="吞吐" a-label="发送" b-label="接收" unit="kbps" a-color="#9b8cff" :points="store.tput" />
          <SparkChart title="帧率" a-label="视频" b-label="指针" unit="fps" a-color="#f0b429" b-color="#3ee0c5" :points="store.fps" />
          <BBRStrip />
        </div>
        <aside class="flex flex-col gap-4 lg:col-span-4">
          <section class="panel p-4">
            <h3 class="text-xs uppercase tracking-[0.2em] text-mute">模拟丢包</h3>
            <div class="mt-3 grid grid-cols-4 gap-2">
              <button
                v-for="p in ['0', '10', '30', '50']"
                :key="p"
                type="button"
                class="rounded-md border py-2 font-mono text-sm"
                :class="store.preset === p ? 'border-cyan bg-cyan/10 text-cyan' : 'border-line text-mute'"
                :disabled="!store.connected"
                @click="changePreset(p)"
              >{{ p }}%</button>
            </div>
            <p class="mt-3 font-mono text-[11px] text-mute">标记 {{ store.marks.map(m => m.label).join(' · ') || '尚无' }}</p>
            <label class="mt-4 flex items-center gap-2 font-mono text-xs text-mute">
              <input type="checkbox" :checked="store.bbrEnabled" :disabled="!store.connected" @change="link.setBBR(($event.target as HTMLInputElement).checked)" />
              应用层 BBR 令牌桶
            </label>
          </section>
          <section class="panel p-4">
            <h3 class="text-xs uppercase tracking-[0.2em] text-mute">通道队列</h3>
            <ul class="mt-3 space-y-2 font-mono text-xs">
              <li v-for="q in store.queues" :key="q.channel" class="flex justify-between text-mute">
                <span>{{ q.channel }}</span>
                <span>q={{ q.depth }} drop={{ q.drops }}</span>
              </li>
              <li v-if="!store.queues.length" class="text-mute">等待指标…</li>
            </ul>
          </section>
          <section class="panel p-4 font-mono text-xs text-mute">
            <div>WT {{ store.wtUrl || '—' }}</div>
            <div class="mt-1">BBR 层次 {{ store.bbrLayer }}</div>
            <div class="mt-1">maxDatagram {{ store.maxDatagram }}</div>
          </section>
        </aside>
      </div>

      <div class="mt-4 grid w-full grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
        <FileLane />
        <CursorCanvas />
        <MediaLane ref="media" />
      </div>
    </main>
  </div>
</template>
