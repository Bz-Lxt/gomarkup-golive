import { defineStore } from 'pinia'

export type Point = { t: number; a: number; b: number }
export type BBRPoint = { t: number; state: string; pacing: number; btl: number; gain: number }

const MAX = 1000

function push<T>(arr: T[], v: T) {
  arr.push(v)
  if (arr.length > MAX) arr.splice(0, arr.length - MAX)
}

export const useConsole = defineStore('console', {
  state: () => ({
    supported: typeof (globalThis as { WebTransport?: unknown }).WebTransport === 'function',
    connected: false,
    connecting: false,
    sessionId: '',
    wtUrl: '',
    certHours: 0,
    quicVersion: 0,
    gso: false,
    datagram: false,
    maxDatagram: 1024,
    bbrLayer: 'application-scheduler',
    bbrEnabled: true,
    preset: '0',
    rtt: [] as Point[],
    tput: [] as Point[],
    fps: [] as Point[],
    bbr: [] as BBRPoint[],
    lastMetrics: null as Record<string, unknown> | null,
    queues: [] as { channel: string; depth: number; drops: number }[],
    marks: [] as { t: number; label: string }[],
    toasts: [] as { id: number; kind: 'info' | 'error' | 'ok'; text: string }[],
    toastSeq: 1,
    fileProgress: 0,
    fileAck: '',
    audioLevel: 0,
    audioDrops: 0,
    cursorRemote: [] as { x: number; y: number; t: number }[],
    videoFps: 0,
    modal: null as null | { title: string; body: string; confirm: string; onOk: () => void },
  }),
  actions: {
    toast(kind: 'info' | 'error' | 'ok', text: string) {
      const id = this.toastSeq++
      this.toasts.push({ id, kind, text })
      setTimeout(() => this.dismiss(id), 5000)
    },
    dismiss(id: number) {
      this.toasts = this.toasts.filter((x) => x.id !== id)
    },
    ask(title: string, body: string, confirm: string, onOk: () => void) {
      this.modal = { title, body, confirm, onOk }
    },
    closeModal() {
      this.modal = null
    },
    pushMetrics(m: Record<string, unknown>) {
      this.lastMetrics = m
      const t = Number(m.ts) || Date.now()
      push(this.rtt, { t, a: Number(m.smoothed_rtt_ms) || 0, b: Number(m.app_rtt_ms) || 0 })
      push(this.tput, { t, a: (Number(m.send_bps) || 0) / 1000, b: (Number(m.recv_bps) || 0) / 1000 })
      push(this.fps, { t, a: Number(m.video_fps) || 0, b: Number(m.cursor_fps) || 0 })
      const bbr = (m.bbr || {}) as Record<string, unknown>
      push(this.bbr, {
        t,
        state: String(bbr.state ?? ''),
        pacing: Number(bbr.pacing_bps) || 0,
        btl: Number(bbr.btlbw_bps) || 0,
        gain: Number(bbr.pacing_gain) || 0,
      })
      this.sessionId = String(m.session_id || this.sessionId)
      this.quicVersion = Number(m.quic_version) || 0
      this.gso = Boolean(m.gso)
      this.datagram = Boolean(m.datagram_remote) && Boolean(m.datagram_local)
      this.maxDatagram = Number(m.max_datagram) || 1024
      this.audioDrops = Number(m.dropped_audio) || 0
      this.videoFps = Number(m.video_fps) || 0
      const sch = (m.scheduler || {}) as { queues?: { channel: string; depth: number; drops: number }[] }
      this.queues = sch.queues || []
    },
    mark(label: string) {
      this.marks.push({ t: Date.now(), label })
      if (this.marks.length > 20) this.marks.shift()
    },
  },
})
