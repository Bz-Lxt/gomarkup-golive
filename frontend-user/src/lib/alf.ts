import { crc32ieee } from './crc32'

export const MAGIC = 0x474c4146
export const VERSION = 1
export const HEADER = 35

export const CH = { signal: 0, audio: 1, cursor: 2, video: 3, file: 4 } as const
export const KIND = { bidi: 1, uni: 2, datagram: 3 } as const

export type ChannelName = keyof typeof CH

export interface Frame {
  channel: number
  kind: number
  priority: number
  seq: bigint
  pts: bigint
  flags: number
  fragIdx: number
  fragTotal: number
  payload: Uint8Array
}

export const FLAG_EOM = 1
export const FLAG_PING = 4
export const FLAG_PONG = 8

function putU64(view: DataView, off: number, v: bigint) {
  view.setBigUint64(off, v, false)
}

export function encode(f: Frame): Uint8Array {
  const buf = new Uint8Array(HEADER + f.payload.length)
  const view = new DataView(buf.buffer)
  view.setUint32(0, MAGIC, false)
  buf[4] = VERSION
  buf[5] = f.channel
  buf[6] = f.kind
  buf[7] = f.priority
  putU64(view, 8, f.seq)
  putU64(view, 16, f.pts)
  buf[24] = f.flags
  view.setUint16(25, f.fragIdx, false)
  view.setUint16(27, f.fragTotal, false)
  view.setUint16(29, f.payload.length, false)
  buf.set(f.payload, HEADER)
  const crc = crc32ieee(concat(buf.subarray(0, 31), f.payload))
  view.setUint32(31, crc, false)
  return buf
}

export function decode(raw: Uint8Array): Frame {
  if (raw.length < HEADER) throw new Error('alf: short header')
  const view = new DataView(raw.buffer, raw.byteOffset, raw.byteLength)
  if (view.getUint32(0, false) !== MAGIC) throw new Error('alf: bad magic')
  if (raw[4] !== VERSION) throw new Error('alf: bad version')
  const plen = view.getUint16(29, false)
  if (HEADER + plen !== raw.length) throw new Error('alf: length mismatch')
  const payload = raw.subarray(HEADER, HEADER + plen)
  const want = view.getUint32(31, false)
  const got = crc32ieee(concat(raw.subarray(0, 31), payload))
  if (want !== got) throw new Error('alf: checksum')
  return {
    channel: raw[5],
    kind: raw[6],
    priority: raw[7],
    seq: view.getBigUint64(8, false),
    pts: view.getBigUint64(16, false),
    flags: raw[24],
    fragIdx: view.getUint16(25, false),
    fragTotal: view.getUint16(27, false),
    payload: payload.slice(),
  }
}

export function split(base: Omit<Frame, 'payload' | 'fragIdx' | 'fragTotal'>, payload: Uint8Array, max: number): Frame[] {
  if (payload.length === 0) {
    return [{ ...base, fragIdx: 0, fragTotal: 1, flags: base.flags | FLAG_EOM, payload: new Uint8Array() }]
  }
  const n = Math.ceil(payload.length / max)
  const out: Frame[] = []
  for (let i = 0; i < n; i++) {
    const start = i * max
    const end = Math.min(start + max, payload.length)
    out.push({
      ...base,
      fragIdx: i,
      fragTotal: n,
      flags: i === n - 1 ? base.flags | FLAG_EOM : base.flags & ~FLAG_EOM,
      payload: payload.subarray(start, end),
    })
  }
  return out
}

export async function writeStream(w: WritableStream<Uint8Array>, f: Frame) {
  const raw = encode(f)
  const prefix = new Uint8Array(4)
  new DataView(prefix.buffer).setUint32(0, raw.length, false)
  const writer = w.getWriter()
  await writer.write(prefix)
  await writer.write(raw)
  writer.releaseLock()
}

class BytePump {
  private leftover = new Uint8Array(0)
  constructor(private reader: ReadableStreamDefaultReader<Uint8Array>) {}

  async read(n: number): Promise<Uint8Array> {
    const out = new Uint8Array(n)
    let off = 0
    while (off < n) {
      if (this.leftover.length > 0) {
        const take = Math.min(this.leftover.length, n - off)
        out.set(this.leftover.subarray(0, take), off)
        this.leftover = this.leftover.subarray(take)
        off += take
        continue
      }
      const { value, done } = await this.reader.read()
      if (done || !value) throw new Error('alf: truncated stream')
      this.leftover = value
    }
    return out
  }
}

export async function* iterateFrames(r: ReadableStream<Uint8Array>): AsyncGenerator<Frame> {
  const reader = r.getReader()
  const pump = new BytePump(reader)
  try {
    for (;;) {
      const head = await pump.read(4)
      const n = new DataView(head.buffer, head.byteOffset, 4).getUint32(0, false)
      if (n < HEADER || n > 2 << 20) throw new Error('alf: framed size')
      const body = await pump.read(n)
      yield decode(body)
    }
  } finally {
    reader.releaseLock()
  }
}

export async function readStream(r: ReadableStream<Uint8Array>): Promise<Frame> {
  const it = iterateFrames(r)
  const first = await it.next()
  if (first.done || !first.value) throw new Error('alf: empty stream')
  return first.value
}

function concat(a: Uint8Array, b: Uint8Array): Uint8Array {
  const o = new Uint8Array(a.length + b.length)
  o.set(a)
  o.set(b, a.length)
  return o
}

export function maxDatagramPayload(cap = 1024): number {
  return Math.max(0, Math.min(cap, 1024) - HEADER)
}

export function nowPts(): bigint {
  return BigInt(Date.now())
}
