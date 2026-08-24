import { CH, KIND, encode, decode, writeStream, iterateFrames, maxDatagramPayload, nowPts, FLAG_EOM, type Frame } from './alf'
import { b64rawToBuffer, sha256Hex } from './hash'
import { log } from './logger'
import { useConsole } from '../stores/console'

export type Fingerprint = {
  algorithm: string
  base64_raw: string
  wt_url: string
  valid_hours: number
}

export class LiveLink {
  private wt: WebTransport | null = null
  private seq = 1n
  private closed = false
  private onCursor: ((x: number, y: number) => void) | null = null

  async connect() {
    const store = useConsole()
    if (!store.supported) throw new Error('浏览器不支持 WebTransport')
    store.connecting = true
    const fp = (await (await fetch('/api/v1/cert-fingerprint')).json()) as Fingerprint
    if (fp.algorithm !== 'sha-256') throw new Error('fingerprint algorithm 必须是小写 sha-256')
    store.wtUrl = fp.wt_url
    store.certHours = fp.valid_hours
    const hash = b64rawToBuffer(fp.base64_raw)
    const wt = new WebTransport(fp.wt_url, {
      serverCertificateHashes: [{ algorithm: 'sha-256', value: hash }],
    })
    await wt.ready
    this.wt = wt
    store.connected = true
    store.connecting = false
    this.pumpDatagrams()
    this.pumpIncomingBidi()
    await this.sendSignal({ type: 'hello', payload: { room: 'default', mode: 'echo', client: 'vue3' } })
    log.info('webtransport ready', { url: fp.wt_url })
  }

  async disconnect() {
    this.closed = true
    this.wt?.close()
    this.wt = null
    useConsole().connected = false
  }

  onRemoteCursor(fn: (x: number, y: number) => void) {
    this.onCursor = fn
  }

  async sendSignal(env: { type: string; id?: string; payload?: unknown }) {
    const body = new TextEncoder().encode(JSON.stringify(env))
    const f: Frame = {
      channel: CH.signal, kind: KIND.bidi, priority: 0,
      seq: this.seq++, pts: nowPts(), flags: FLAG_EOM, fragIdx: 0, fragTotal: 1, payload: body,
    }
    if (!this.wt) return
    const stream = await this.wt.createBidirectionalStream()
    await writeStream(stream.writable, f)
    void stream.writable.close()
    void this.readOneBidi(stream.readable)
  }

  async setPreset(preset: string) {
    await this.sendSignal({ type: 'netem.set', payload: { preset } })
    useConsole().preset = preset
    useConsole().mark(`${preset}%`)
  }

  async setBBR(enabled: boolean) {
    await this.sendSignal({ type: 'bbr.set', payload: { enabled } })
    useConsole().bbrEnabled = enabled
  }

  async sendCursor(x: number, y: number) {
    const payload = new TextEncoder().encode(JSON.stringify({ x, y, t: Date.now() }))
    await this.sendDatagram(CH.cursor, 1, payload)
  }

  async sendAudio(pcm: Uint8Array) {
    await this.sendDatagram(CH.audio, 1, pcm)
  }

  async sendVideo(jpeg: Uint8Array) {
    if (!this.wt) return
    const f: Frame = {
      channel: CH.video, kind: KIND.uni, priority: 2,
      seq: this.seq++, pts: nowPts(), flags: FLAG_EOM, fragIdx: 0, fragTotal: 1, payload: jpeg,
    }
    const raw = encode(f)
    const u = await this.wt.createUnidirectionalStream()
    const w = u.getWriter()
    await w.write(raw)
    await w.close()
  }

  async sendFile(file: File) {
    const store = useConsole()
    const buf = await file.arrayBuffer()
    const sha = await sha256Hex(buf)
    await this.sendSignal({
      type: 'file.begin',
      payload: { name: file.name, size: file.size, sha256: sha, chunks: 1 },
    })
    const bytes = new Uint8Array(buf)
    const max = 8 * 1024
    if (!this.wt) return
    const stream = await this.wt.createBidirectionalStream()
    const chunks = Math.ceil(bytes.length / max) || 1
    for (let i = 0; i < chunks; i++) {
      const slice = bytes.subarray(i * max, Math.min(bytes.length, (i + 1) * max))
      const f: Frame = {
        channel: CH.file, kind: KIND.bidi, priority: 3,
        seq: this.seq++, pts: nowPts(),
        flags: i === chunks - 1 ? FLAG_EOM : 0,
        fragIdx: i, fragTotal: chunks, payload: slice,
      }
      await writeStream(stream.writable, f)
      store.fileProgress = Math.round(((i + 1) / chunks) * 100)
    }
    void stream.writable.close()
    await this.sendSignal({ type: 'file.done', payload: { name: file.name } })
  }

  async ping() {
    await this.sendSignal({ type: 'ping', payload: { client_ts: Date.now(), seq: Number(this.seq) } })
  }

  private async sendDatagram(ch: number, prio: number, payload: Uint8Array) {
    if (!this.wt) return
    const max = maxDatagramPayload(useConsole().maxDatagram)
    const base = { channel: ch, kind: KIND.datagram, priority: prio, seq: this.seq++, pts: nowPts(), flags: 0 }
    const parts = payload.length > max
      ? [{ ...base, fragIdx: 0, fragTotal: 1, flags: FLAG_EOM, payload: payload.subarray(0, max) }]
      : [{ ...base, fragIdx: 0, fragTotal: 1, flags: FLAG_EOM, payload }]
    const w = this.wt.datagrams.writable.getWriter()
    try {
      for (const p of parts) {
        await w.write(encode(p as Frame))
      }
    } finally {
      w.releaseLock()
    }
  }

  private async pumpDatagrams() {
    if (!this.wt) return
    const reader = this.wt.datagrams.readable.getReader()
    try {
      while (!this.closed) {
        const { value, done } = await reader.read()
        if (done || !value) break
        this.onRaw(value)
      }
    } catch (e) {
      log.warn('datagram pump', { err: String(e) })
    }
  }

  private async pumpIncomingBidi() {
    if (!this.wt) return
    const incoming = (this.wt as unknown as { incomingBidirectionalStreams: ReadableStream }).incomingBidirectionalStreams
    const reader = incoming.getReader()
    try {
      while (!this.closed) {
        const { value, done } = await reader.read()
        if (done || !value) break
        void this.readOneBidi(value.readable)
      }
    } catch (e) {
      log.warn('bidi pump', { err: String(e) })
    }
  }

  private async readOneBidi(readable: ReadableStream<Uint8Array>) {
    try {
      for await (const f of iterateFrames(readable)) {
        this.onFrame(f)
      }
    } catch (e) {
      log.debug('bidi frame', { err: String(e) })
    }
  }

  private onRaw(raw: Uint8Array) {
    try {
      this.onFrame(decode(raw))
    } catch (e) {
      log.debug('dgram decode', { err: String(e) })
    }
  }

  private onFrame(f: Frame) {
    const store = useConsole()
    if (f.channel === CH.signal) {
      try {
        const env = JSON.parse(new TextDecoder().decode(f.payload)) as {
          type: string
          payload?: Record<string, unknown>
        }
        if (env.type === 'metrics' && env.payload) store.pushMetrics(env.payload)
        if (env.type === 'welcome' && env.payload) {
          store.sessionId = String(env.payload.session_id || '')
          store.bbrLayer = String(env.payload.bbr_layer || store.bbrLayer)
          store.toast('ok', `会话 ${store.sessionId.slice(0, 8)} 已建立`)
        }
        if (env.type === 'pong' && env.payload) {
          const rtt = Date.now() - Number(env.payload.client_ts)
          store.pushMetrics({ ...(store.lastMetrics || {}), app_rtt_ms: rtt, ts: Date.now() })
        }
        if (env.type === 'file.ack' && env.payload) {
          store.fileAck = env.payload.match ? `校验一致 ${env.payload.sha256}` : `校验失败 ${env.payload.error || ''}`
          store.toast(env.payload.match ? 'ok' : 'error', store.fileAck)
        }
        if (env.type === 'error' && env.payload) store.toast('error', String(env.payload.message || '信号错误'))
        if (env.type === 'netem.ok') store.toast('info', '损伤配置已热切换')
      } catch (e) {
        log.warn('signal json', { err: String(e) })
      }
      return
    }
    if (f.channel === CH.cursor) {
      try {
        const p = JSON.parse(new TextDecoder().decode(f.payload)) as { x: number; y: number }
        this.onCursor?.(p.x, p.y)
        store.cursorRemote.push({ x: p.x, y: p.y, t: Date.now() })
        if (store.cursorRemote.length > 400) store.cursorRemote.splice(0, 80)
      } catch {
        /* ignore */
      }
    }
    if (f.channel === CH.audio) {
      let sum = 0
      for (const b of f.payload) sum += Math.abs(b - 128)
      store.audioLevel = Math.min(1, sum / (f.payload.length * 128))
    }
  }
}

export const link = new LiveLink()
