# GoLive API

> 时区：GMT+8，用户可见时间格式 `yyyy-MM-dd HH:mm:ss`
> 错误体统一：`{ "code", "message", "timestamp" }`

## 1. TCP REST（页面同源）

Base：`http://localhost:19443`

### GET /api/v1/health

真实依赖探测，禁止硬编码 ok。

```
200 { "status":"ok", "time":"2026-08-23 16:00:00", "udp_listening":true, "cert_hours_remaining":300, "sessions":0, "rooms":1, "bbr_layer":"application-scheduler" }
503 当 UDP 未监听或证书剩余 < 1 小时
```

### GET /api/v1/cert-fingerprint

```
200 {
  "algorithm":"sha-256",          // 必须小写
  "hex":"...",
  "base64":"...",
  "base64_raw":"...",             // 推荐给 WebTransport
  "not_before":"...",
  "not_after":"...",
  "curve":"P-256",
  "valid_hours":300,
  "wt_url":"https://localhost:19444/webtransport",
  "note":"hash is SHA-256 of leaf DER, not SPKI"
}
```

### GET /api/v1/config

公开运行时配置（端口、通道矩阵、BBR 层次、netem 预设名）。

### GET /api/v1/netem

当前上下行损伤快照。

### PUT /api/v1/netem

```
{ "preset":"30" }
或
{ "uplink": { "loss_pct":30, "delay_ms":20, "jitter_ms":12 }, "downlink": { ... } }

422 bad_preset / bad_uplink / missing
```

### GET /api/v1/sessions

当前 WebTransport 会话列表。

## 2. WebTransport

`https://localhost:19444/webtransport`

握手前必须：

```js
const fp = await fetch('/api/v1/cert-fingerprint').then(r => r.json())
const hash = Uint8Array.from(atob(fp.base64_raw + '=='.slice((fp.base64_raw.length % 4) || 4)), c => c.charCodeAt(0))
const wt = new WebTransport(fp.wt_url, {
  serverCertificateHashes: [{ algorithm: 'sha-256', value: hash.buffer }]
})
await wt.ready
```

`algorithm` 必须字面量 `sha-256`。不要传 `allowPooling`。

## 3. ALF 帧（35 字节头）

| Offset | 字段 | 类型 |
|---|---|---|
| 0 | magic `GLAF` | u32be `0x474C4146` |
| 4 | version | u8 `1` |
| 5 | channel_id | u8 0=signal 1=audio 2=cursor 3=video 4=file |
| 6 | stream_kind | u8 1=bidi 2=uni 3=datagram |
| 7 | priority | u8 0..3 |
| 8 | seq | u64be |
| 16 | pts | i64be unix ms |
| 24 | flags | u8 bit0=EOM bit2=ping bit3=pong |
| 25 | frag_idx | u16be |
| 27 | frag_total | u16be ≥1 |
| 29 | payload_len | u16be |
| 31 | crc32 IEEE | u32be over header[0:31]+payload |
| 35 | payload | bytes |

Datagram 分片预算：`min(session, 1024) - 35`。

可靠流（signal / file）外加 4 字节长度前缀（整帧含头）。

## 4. 信令 JSON（signal 通道 payload）

| type | 方向 | payload |
|---|---|---|
| hello | C→S | `{room, client, mode:"echo"\|"room"}` |
| welcome | S→C | `{session_id, room, mode, wt_url, max_datagram, channels, bbr_layer, server_time}` |
| metrics | S→C 10Hz | 见 `metrics.Snapshot`（含双源 RTT、吞吐、BBR、队列） |
| netem.set / netem.ok | C↔S | preset 或 uplink/downlink |
| bbr.set / bbr.ok | C↔S | `{enabled}` |
| ping / pong | C↔S | `{client_ts, seq}` / `+ server_ts` |
| file.begin / file.ack | C↔S | `{name,size,sha256}` / `{received,sha256,match}` |
| error | S→C | `{code,message}` |

## 5. 错误码

| code | HTTP | 含义 |
|---|---|---|
| bad_json | 400 | 请求体无法解析 |
| bad_preset | 422 | 预设不是 0/10/30/50 |
| missing | 422 | 缺 preset 或双端 profile |
| no_ui | 404 | 前端未嵌入 |

WebTransport origin 不在白名单：握手失败，TCP 侧无此错误码。
