# 契约核实记录（Contract Gate — PM 前置执行）

> 核实时间：2026-08-23 (GMT+8) · 执行人：PM Agent
> 本项目**不含任何计费外部 API**，故 SOP v13 §Phase 3 Contract Gate 的"一次真实调用"对象转为
> **依赖库公开 API** 与 **浏览器平台 API**。这两者恰是本项目最容易"对着想象的接口生成上千行代码"的地方。
> 状态标记：`VERIFIED` = 已用 `go doc` / 规范原文实测确认；`ASSUMED` = 未实测，Phase 3 必须复验。

## 0. 宿主环境（实测）

| 项 | 实测值 |
|---|---|
| Go | `go1.25.12 darwin/arm64` |
| Docker | `29.6.1` |
| Docker Compose | `v5.1.4` |
| GOPROXY | `https://proxy.golang.org,direct`（容器内须改 `goproxy.cn`，见 devops 记忆） |

## 1. 依赖版本（实测，锁定）

| 模块 | 版本 | 发布时间 | 状态 |
|---|---|---|---|
| `github.com/quic-go/quic-go` | `v0.61.0` | 2026-07-21 | VERIFIED |
| `github.com/quic-go/webtransport-go` | `v0.12.0` | 2026-07-27 | VERIFIED |

## 2. quic-go v0.61.0 —— 关键 API 形状

### 2.1 遥测入口：`Conn.ConnectionStats()` ✅ VERIFIED

```go
func (c *quic.Conn) ConnectionStats() quic.ConnectionStats

type ConnectionStats struct {
    MinRTT, LatestRTT, SmoothedRTT, MeanDeviation time.Duration
    BytesSent, PacketsSent       uint64
    BytesReceived, PacketsReceived uint64
    BytesLost, PacketsLost       uint64  // 注意：非单调递增
}
```

**结论**：RTT / 吞吐 / 丢包三类指标**全部有真实来源，无需任何 mock**。这是"弱网对抗大屏"的权威数据源。
**坑**：`BytesLost` / `PacketsLost` 文档明确"不单调递增"（被判丢失的包后续可能又收到），前端画累计曲线前必须做单调化或改画瞬时速率。

### 2.2 丢包注入点：`net.PacketConn` ✅ VERIFIED

```go
type quic.Transport struct { Conn net.PacketConn; ... }
func (s *webtransport.Server) Serve(conn net.PacketConn) error
```

**结论**：可以自建 `net.PacketConn` 装饰器，在 **UDP 数据包粒度**做真实丢弃/延迟/抖动/乱序。
QUIC 的丢包检测、ACK、重传、拥塞控制会**真实反应** → 这是真实网络仿真，不是 mock。
**推论**：不需要 `tc netem`，不需要 `NET_ADMIN` capability，跨平台行为一致。

### 2.3 拥塞控制：**不可插拔** ❌ VERIFIED（关键否定结论）

实测证据：
- `go doc -all github.com/quic-go/quic-go | rg -i "congestion|bbr|pacing|cubic|reno"` → **零命中**。
- `quic.Config` 全字段实测：**无任何 CC / pacing 注入字段**。
- `quic-go@v0.61.0/internal/congestion/` 目录实际内容：
  `bandwidth.go cubic.go cubic_sender.go hybrid_slow_start.go pacer.go interface.go`
  → **只有 Cubic，根本没有 BBR 实现**；且位于 `internal/`，外部包**无法 import**。

**结论**：原始需求"深度定制底层连接的拥塞控制算法适配（如 BBR 策略微调）"**在传输层不可实现**。
处置方案见 `docs/Requirements.md` 矛盾 C-01。

### 2.4 Tracer 已换代 ⚠️ VERIFIED（API 漂移陷阱）

```go
// v0.61.0 实际签名
Tracer func(ctx context.Context, isClient bool, connID ConnectionID) qlogwriter.Trace
```

旧的 `github.com/quic-go/quic-go/logging` 包 + `*logging.ConnectionTracer` 结构体回调**已不存在**
（实测 `go doc .../logging.ConnectionTracer` → `no required module provides package`）。
新机制为 `qlogwriter.Trace → AddProducer() → Recorder.RecordEvent(Event)`，`Event` 需实现 `Name()` + `Encode(*jsontext.Encoder, time.Time)`。

**警告给 Phase 3**：网络上绝大多数 quic-go 教程与模型记忆里的 `logging.ConnectionTracer{UpdatedMetrics: ...}` 写法**编译不过**。
遥测优先走 §2.1 的 `ConnectionStats()` 轮询，qlog 仅作为可选的离线诊断产物。

### 2.5 其他已确认字段

- `quic.Config.EnableDatagrams bool` ✅ —— Datagram（RFC 9221）开关，必须置 true。
- `quic.ConnectionState.GSO bool` ✅ —— 说明 quic-go 会启用 GSO；Docker 虚拟网络下已知有兼容问题，须 `QUIC_GO_DISABLE_GSO=1` 兜底。
- `quic.Config.InitialPacketSize uint16`、`DisablePathMTUDiscovery bool` ✅ —— 可用于 MTU 相关调优。
- `quic.Config` 无 `Allow0RTT` 之外的服务端 CC 相关旋钮。

## 3. webtransport-go v0.12.0 —— 能力边界

### 3.1 已由库实现（我们**不要**重写）✅ VERIFIED

- `Server{ H3 *http3.Server; Config *Config; ApplicationProtocols []string; ReorderingTimeout; CheckOrigin }`
- `ConfigureHTTP3Server(*http3.Server)`、`Server.Upgrade(w, r) (*Session, error)`
- CONNECT 握手、SETTINGS 协商、capsule/帧层解析、流与 session 的关联（含乱序到达缓冲）

### 3.2 已确认可用的原语 ✅ VERIFIED

```go
(*Session).OpenStream() / OpenStreamSync(ctx)        // 双向流
(*Session).OpenUniStream() / OpenUniStreamSync(ctx)  // 单向流（发）
(*Session).AcceptStream(ctx) / AcceptUniStream(ctx)  // 双向流 / 单向流（收）
(*Session).SendDatagram(b) / ReceiveDatagram(ctx)    // 不可靠通道
(*Session).SessionState() SessionState
(*Session).CloseWithError(code, msg)
```

**结论**：需求中"双向流与单向流解耦分发"+"Datagram"的**全部传输原语齐备**。
`ReorderingTimeout` 已覆盖"流先于 CONNECT 到达"的乱序场景，无需自行实现。

### 3.3 `CheckOrigin` 默认行为 ⚠️ VERIFIED（会导致连接被拒）

库注释原文：`If unset, a safe default is used: If the Origin header is set, it is checked that it matches the request's Host header.`

**推论**：前端页面由 `http://localhost:8081` 提供，WebTransport 连 `https://localhost:8082`
→ `Origin=http://localhost:8081` ≠ `Host=localhost:8082` → **默认策略直接拒绝握手**。
必须实现**显式 origin 白名单**（读配置），**禁止** `return true` 图省事（CSRF）。

## 4. 浏览器平台 API —— 硬约束

### 4.1 `serverCertificateHashes` 证书要求 ✅ VERIFIED（W3C 规范 + Chromium 源码）

来源：W3C WebTransport 规范 / MDN / `chromium/src/net/quic/dedicated_web_transport_http3_client.cc`
（源码常量 `kCustomCertificateMaxValidityDays = 14`）

| 约束 | 细节 | 违反后果 |
|---|---|---|
| 哈希对象 | SHA-256 over **leaf 证书 DER 全文** | 错哈希 SPKI → **静默失败，无诊断信息** |
| 总有效期 ≤ 14 天 | 是 `notBefore`→`notAfter` **整段跨度**，非剩余天数 | 90 天证书**任何引擎都无法 pin** |
| 密钥算法 | **ECDSA P-256 (secp256r1)**；规范明令**排除 RSA** | RSA → Chrome / Firefox 直接拒绝 |
| algorithm 字段 | 必须**小写** `"sha-256"` | WebKit / Firefox 按字面比较，`"SHA-256"` → **静默永不匹配** |
| `allowPooling` | 必须为 `false`（默认值） | 为 true 则抛 `NotSupportedError` |
| 不 pin 时的根证书 | Chrome 要求根为**公共已知根**，mkcert / 私有 CA 一律失败（`ERR_QUIC_CERT_ROOT_NOT_KNOWN`） | 故**必须 pin**，不得依赖 mkcert |

### 4.2 前端**不能**依赖 `getStats()` 取指标 ❌ VERIFIED（直接改变架构）

- Chrome stable：`getStats()` 与 `congestionControl` **未启用**。
- Firefox：`getStats()` **抛异常**。
- Chrome：`maxDatagramSize` **硬编码 1024**，且不随协商出的 path MTU 更新。
- Chrome：`allowPooling` / `requireUnreliable` 字典成员不存在，WebIDL 静默丢弃未知成员 → 传了等于没传且无报错。

**结论**：需求"实时展示 RTT / 吞吐量 / 帧率曲线"的数据**不能**来自浏览器 API。
必须改为「服务端 `ConnectionStats()` 下推 + 客户端应用层自测」双源，见 Requirements 矛盾 C-04。
**Datagram 分片预算必须按 1024 字节算，不是 1200。**

### 4.3 兼容性基线 ✅ VERIFIED（caniuse / web-features）

WebTransport 已于 **2026-03-24 进入 Baseline (low)**：
Chrome 97+ / Edge 97+ / Firefox 114+ / **Safari 26.4+** / iOS Safari 26.4+。

**结论**：无需为 Safari 做功能阉割，但仍须实现 `if (!window.WebTransport)` 能力探测与友好降级提示
（覆盖 Safari ≤26.3、iOS ≤26.3 及各类内置 WebView）。
**Safari/Firefox 特有坑**：`algorithm` 字符串大小写敏感（见 §4.1），前端必须硬编码小写。

## 5. 遗留 `ASSUMED` 项（Phase 3 必须复验）

| # | 假设 | 复验方式 |
|---|---|---|
| A-1 | Docker Desktop for Mac 的 UDP 端口转发可稳定承载 QUIC（含 1200+ 字节包不被分片吞掉） | Phase 4 冒烟：容器内起服务，宿主 Go 客户端完成握手 + 1 MB 流传输校验 SHA-256 |
| A-2 | `QUIC_GO_DISABLE_GSO=1` 足以规避容器内 GSO 报错 | 对照实验：置 0 / 置 1 各跑一次握手，记录 `ConnectionState.GSO` 与错误 |
| A-3 | 应用层 BBR 的 pacing 输出能实测影响吞吐曲线（而非仅是好看的数字） | 固定 30% 丢包，对比 BBR 开 / 关两组吞吐与 RTT 曲线，须有可辨差异 |
| A-4 | 自签 ECDSA P-256 13 天证书能被目标 Chrome 成功 pin | Phase 4 E2E：Playwright 起 Chromium 实连，断言 `transport.ready` resolve |

## 6. 成本

**本项目全链路零计费外部 API。** 预期 QA 每轮花费 **¥0**。
无需实现预算上限 / 按次计费记账（SOP v13 Phase 3 §4 成本护栏在此项目不适用，
但**窄重试**原则仍适用于 QUIC 连接建立：仅对瞬态网络错误重试，不对证书校验失败 / origin 拒绝重试）。
