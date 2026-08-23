# 需求规格说明书 (SSOT)

> 项目代号：**GoLive** — 现代 HTTP/3 与 WebTransport 实时流媒体控制台
> 文档版本：v1.0 (Requirements Frozen)
> 冻结时间：2026-08-23 (GMT+8)
> 产出人：PM Agent (Alkaid-SOP v13.0)
> 原始需求：见 `docs/.meta/original_prompt.md`（逐字保存，冲突时以其为准）
> 技术契约核实：见 `docs/.meta/api_contracts.md`（本文件所有技术结论的实测依据）

---

## 1. 废除评估结论（SOP Step 1）

| # | 判据 | 结论 | 依据 |
|---|---|---|---|
| 1 | 不完整 / 模糊 | **通过** | 需求含明确主题、前后端职责、文件数与代码量预算，无缺失附件依赖 |
| 2 | Windows 独占 | **通过** | Go + Docker，全链路 Linux 容器内运行；宿主实测 `darwin/arm64` |
| 3 | 规模评估 | **通过（Tier 1）** | 6500–9000 LoC < 10,000 → 直接接受。虽在下限区间但技术密度高，仍建议 Roadmap 划分阶段边界 |
| 4 | 外部依赖 | **通过** | 无付费 API、无实时事实性数据（股价 / 赛果 / 路况）。RTT / 吞吐 / 丢包三项指标均为**本机真实测量值**，非外部事实，见 §4.1 |
| 5 | 专有 / 付费软件 | **通过** | `quic-go`、`webtransport-go` 均为开源；无商业授权依赖 |

### 结论：**ACCEPT（接受立项）**

---

## 2. 项目定位与业务目标

构建一个可 `docker compose up` 一键启动的**全栈实时传输实验台**，用于**可验证地**回答一个问题：

> 在 10% / 30% / 50% 丢包的恶劣网络下，QUIC + WebTransport 相比传统传输能保住什么、保不住什么？

三条业务主线：

1. **弱网对抗测试大屏** — 在真实注入的丢包 / 延迟 / 抖动下，实时呈现 RTT、吞吐量、帧率三类曲线。
2. **多通道数据画布** — 可靠通道（Stream）传文件并校验完整性；不可靠通道（Datagram）传鼠标轨迹 / 音频并在画布实时呈现，直观对比两种语义的差异。
3. **多路复用优先级调度** — 视频 / 音频 / 信令三类流量在同一 QUIC 连接上竞争时，验证优先级策略是否真的生效。

**核心交付价值**：不是"能跑"，而是**每个数字都能溯源到真实测量**。

---

## 3. 兼容性与逻辑校验（SOP Step 2）—— 矛盾登记

> 本节是本文档最重要的部分。以下矛盾均**已用实测证据确认**，不是推测。
> 每条给出处置方案，Phase 1–3 必须按此执行，Phase 5 Auditor 按此评判。

### C-01 【阻断级·需重新设计】"深度定制底层拥塞控制 / BBR 策略微调"技术上不可实现

**矛盾**：原需求要求定制 quic-go 底层拥塞控制算法。

**实测证据**（`api_contracts.md` §2.3）：
- `go doc -all quic-go | rg -i "congestion|bbr|pacing|cubic|reno"` → **零命中**，公开 API 完全不暴露拥塞控制。
- `quic.Config` 无任何 CC / pacing 注入字段。
- `quic-go@v0.61.0/internal/congestion/` 实际只有 **Cubic**（`cubic.go`、`cubic_sender.go`、`hybrid_slow_start.go`、`pacer.go`），**根本没有 BBR 实现**；且位于 `internal/`，外部包无法 import。

**已否决方案**：fork quic-go 或 `replace` 指向第三方 BBR 分支。理由：破坏 `go get` 可复现性、引入无维护保证的依赖、上游 API 漂移风险高、且改内核 CC 的工作量本身就会吃掉全部代码预算。

**采纳方案 —— 应用层 BBR 状态机（真实算法，真实接线）**：
1. 实现完整 BBR v1 四状态机：`Startup → Drain → ProbeBW → ProbeRTT`。
2. **输入为真实遥测**：以固定周期从 `Conn.ConnectionStats()` 采集 `SmoothedRTT` / `MinRTT` / `LatestRTT` / `BytesSent` / `BytesLost` / `PacketsLost`，据此估算 `BtlBw`（瓶颈带宽，最大带宽滑动窗口）与 `RTprop`（最小 RTT 传播时延）。
3. **输出真实生效**：算出的 `pacing_rate = pacing_gain × BtlBw` 与 `cwnd = cwnd_gain × BtlBw × RTprop` **作用于应用层多路复用调度器的令牌桶**，真实限制每个优先级队列的出队速率。
4. 状态转移时间线与增益系数在大屏可视化，可开关对比。

**合规性**：满足 Redline 4 Mock 合法性标准 —— 真实实现路径存在且已接线（真实输入 → 真实状态转移 → 真实作用于发送速率）。
**强制披露**：必须在 `README.md` §7 明确写出"BBR 运行在应用层调度器而非 QUIC 传输层内核，原因是 quic-go v0.61.0 未开放 CC 接口"。**隐瞒此层次差异即构成伪实现，Phase 5 应判 FAIL。**

### C-02 【范围澄清】"完整实现 WebTransport 帧格式的解析"

**矛盾**：`webtransport-go v0.12.0` **已经**实现了 CONNECT 握手、SETTINGS 协商、capsule / 帧层解析、流与 session 关联（含乱序缓冲 `ReorderingTimeout`）。从零重写整个 WebTransport 协议栈会：违反"禁止范围漂移"、远超 9000 行预算、且几乎必然与浏览器互操作失败。

**采纳方案 —— 分层切分，各司其职**：
- **WebTransport 传输层**：交由 `webtransport-go`。不重写。
- **应用层帧协议（ALF, Application Layer Framing）**：**由我们完整实现**，这才是"解包"的可交付部分。
  - 帧头字段：`magic` / `version` / `channel_id` / `stream_kind` / `priority` / `seq` / `pts`(GMT+8 单调时钟) / `flags` / `frag_idx` / `frag_total` / `payload_len`。
  - 必须交付：二进制编解码器、分片与重组器、乱序容忍、不可靠通道的重组超时丢弃、校验和、以及**表驱动单元测试**（含畸形输入、截断输入、超长 payload、frag 越界等边界用例）。
  - 约束：外部数据反序列化必须校验结构完整性（字段存在性 / 类型 / 边界），不得仅依赖调用处简单检查（Global 记忆 [Robustness]）。
- **流解耦分发**：`AcceptStream` / `AcceptUniStream` / `ReceiveDatagram` 三条独立接收循环，按 ALF 头的 `channel_id` + `stream_kind` 分发到对应处理器。

### C-03 【阻断级】浏览器证书信任 —— 不做对则前端永远连不上

**实测约束**（W3C 规范 + Chromium `kCustomCertificateMaxValidityDays = 14`）：

| 约束 | 违反后果 |
|---|---|
| SHA-256 必须对 **leaf 证书 DER 全文**（不是 SPKI） | **静默失败，零诊断** |
| 总有效期（`notBefore`→`notAfter` 整段）**≤ 14 天** | 任何引擎都无法 pin |
| 密钥必须 **ECDSA P-256**，规范明令排除 RSA | Chrome / Firefox 直接拒绝 |
| `algorithm` 字符串必须**小写** `"sha-256"` | WebKit / Firefox 按字面比较，**静默永不匹配** |
| `allowPooling` 必须 `false` | 抛 `NotSupportedError` |
| 不 pin 时 Chrome 要求**公共已知根**，mkcert / 私有 CA 一律失败 | `ERR_QUIC_CERT_ROOT_NOT_KNOWN` |

**采纳方案**：
1. 容器启动时**自动生成**自签 **ECDSA P-256** 证书，有效期硬编码 **13 天**（留 1 天安全边界）。
2. 通过 TCP 侧 `GET /api/v1/cert-fingerprint` 返回 leaf DER 的 SHA-256（同时给 hex 与 base64 两种编码）。
3. 前端**先 fetch 指纹，再**构造 `new WebTransport(url, { serverCertificateHashes: [{ algorithm: "sha-256", value: <ArrayBuffer> }] })`。
4. 证书与指纹落在容器内 volume，重启后若剩余有效期不足 1 天则自动重签。
5. **禁止**将证书文件提交进仓库（有效期必然过期，且提交私钥是安全事故）。

### C-04 【阻断级】前端拿不到 RTT —— 指标数据源必须重新设计

**实测证据**：Chrome stable 未启用 `WebTransport.getStats()` 与 `congestionControl`；Firefox 的 `getStats()` **直接抛异常**。

**矛盾**：需求要求前端"实时展示 RTT、吞吐量、帧率曲线"，但浏览器不给数据。

**采纳方案 —— 双源指标，交叉验证**：
- **源 A（传输层真值，权威）**：服务端以 **10 Hz** 采集 `Conn.ConnectionStats()`，经**信令通道**（最高优先级）下推。提供 `SmoothedRTT` / `MinRTT` / `LatestRTT` / `MeanDeviation` / 吞吐（由 `BytesSent`、`BytesReceived` 差分求速率）/ 丢包（`PacketsLost`、`BytesLost` 差分）。
- **源 B（应用层实测，端到端）**：客户端经 **Datagram** 通道发 ping（带单调时钟时间戳），服务端回 pong，客户端算应用层 RTT；帧率由 ALF 帧头 `seq` / `pts` 统计。
- 大屏**并列展示两源**，二者背离即暴露排队延迟 / 应用层积压，本身就是有价值的观测。
- **注意**：`BytesLost` / `PacketsLost` 官方文档明确**不单调递增**（判丢的包可能后续又收到）。前端**禁止**直接画累计曲线，必须画瞬时速率或做单调化处理。

### C-05 【硬约束】Datagram 有效载荷预算为 1024 字节，不是 1200

**实测证据**：Chrome 的 `maxDatagramSize` **硬编码 1024**，且不随协商出的 path MTU 更新。

**采纳方案**：分片上限取 `min(session_max_datagram_size, 1024) - len(ALF_header)`。视频帧必须分片；音频帧与鼠标轨迹应设计为单帧可容纳。不可靠通道**不得重传**，重组器超时（默认 200 ms）即丢弃并计入丢帧统计。

### C-06 【设计决策】丢包模拟必须落在 UDP 包层，不用 tc netem

**矛盾**：浏览器与 localhost 之间不存在真实网络，无处丢包。`tc netem` 需 `NET_ADMIN` capability，且 macOS Docker Desktop 的 VM 网络层会让效果不可控、不可复现。

**实测注入点**：`webtransport.Server.Serve(conn net.PacketConn)` 与 `quic.Transport.Conn net.PacketConn` 均接受自定义 `net.PacketConn`。

**采纳方案**：实现 `internal/netem` 装饰 `net.PacketConn`，在 **UDP 数据包粒度**做丢弃 / 固定延迟 / 抖动 / 乱序，**上行下行独立可配**。
**这是真实网络仿真，不是 mock** —— QUIC 的丢包检测、ACK、重传、拥塞控制会真实反应。丢包率通过信令通道**运行时热切换** 0 / 10% / 30% / 50%，无需重启。
损伤决策必须用**可播种（seeded）**的随机源，保证实验可复现。

### C-07 【DevOps】Docker + QUIC 的已知坑

| 坑 | 处置 |
|---|---|
| QUIC 走 UDP | compose 必须显式 `"port:port/udp"`；**容器内外端口号保持一致**，避免 URL / 证书 / 路径混淆 |
| GSO 在容器虚拟网络下报错（`ConnectionState.GSO` 证实 quic-go 会启用） | 默认注入 `QUIC_GO_DISABLE_GSO=1`，并在 README 说明 |
| 页面加载需 TCP，QUIC 需 UDP | 双端口：TCP 供 Vue 静态资源 + `/api/v1/*`；UDP 供 H3 / WebTransport |
| 自签证书导致页面出现浏览器警告 | 页面走 **plain HTTP `http://localhost:<TCP>`**。`localhost` 被视为 potentially trustworthy，**满足 WebTransport 的 secure context 要求**，从而完全绕开 TLS 警告 |
| 上一条引发 origin 不匹配（`Origin=http://localhost:8081` ≠ `Host=localhost:8082`），库默认 `CheckOrigin` 会**拒绝握手** | 实现**显式 origin 白名单**（读 `ALLOWED_ORIGINS` 配置，默认含 `localhost` 与 `127.0.0.1` 两种写法）。**禁止** `return true` |
| Compose v5.1.4（本机实测版本） | bind mount 禁写 `required: false`（schema 不支持）；`deploy.replicas` 在非 Swarm 不生效 |
| 中国网络下多阶段构建慢 | Dockerfile 内置 `GOPROXY=https://goproxy.cn,direct`、`npm --registry=https://registry.npmmirror.com`、apk 换阿里源 |
| 前端构建 | 必须在镜像内多阶段 build，**禁止** COPY 宿主机 `dist` |
| 时区 | 所有服务 `TZ=Asia/Shanghai`；时间展示统一 `yyyy-MM-dd HH:mm:ss` |

### C-08 【范围澄清】"实时音视频"的媒体来源

**矛盾**：容器内无摄像头 / 麦克风；引入 ffmpeg 转码会炸掉代码预算并偏离"网络网关"主题。

**采纳方案（Scenario A 合法 Mock）**：
- **前端双模式媒体源**：① **合成信号源**（默认）—— Canvas 生成带帧号 / 时间戳 / 运动色块的视频帧，WebAudio 生成音调，保证零硬件也能完整演示；② **真实 `getUserMedia`**（浏览器支持且用户授权时可切换）。
- **后端不做转码**，只做帧级多路复用、优先级调度、回环 / 广播与统计。这正是"新一代网络网关"的本质职责。
- 切换开关必须在 `README.md` §7 记录（Redline 4 合法性前提）。

### C-09 【范围边界】"Mini LiveKit" 措辞需收敛

LiveKit 是 SFU（选择性转发单元），完整 SFU 需要多方订阅关系、带宽估计、simulcast / SVC 层选择、媒体协商 —— 远超 9000 行预算。

**交付边界明确为**：单会话 / 多通道**实时传输网关 + 弱网实验台**。支持两种转发模式：`echo`（回环，用于单人测延迟）与 `room fan-out`（房间内广播，用于多客户端）。
**不实现**：simulcast 层切换、SVC、媒体转码 / 编解码、录制、TURN / ICE、SDP 协商。详见 §7 Out of Scope。

---

## 4. 功能需求（FR）

### 4.1 后端 —— QUIC / HTTP3 服务器

| ID | 需求 | 验收要点 |
|---|---|---|
| FR-B01 | 基于 `quic-go v0.61.0` + `webtransport-go v0.12.0` 的 HTTP/3 服务器，启用 Datagram（`EnableDatagrams=true`） | `ConnectionState.SupportsDatagrams.Remote && .Local` 均为 true |
| FR-B02 | 启动时自动生成 ECDSA P-256 / 13 天自签证书；提供指纹查询端点 | 指纹为 leaf DER 的 SHA-256，hex 与 base64 双编码 |
| FR-B03 | 显式 origin 白名单 `CheckOrigin`，配置驱动 | 白名单外 origin 握手被拒并有结构化日志 |
| FR-B04 | TCP 侧提供 Vue 静态资源 + `/api/v1/*`（健康、指纹、配置、netem 快照） | `/api/v1/health` 反映真实依赖状态，**禁止硬编码 true** |

### 4.2 后端 —— WebTransport 会话与 ALF 帧协议

| ID | 需求 | 验收要点 |
|---|---|---|
| FR-B05 | 会话管理器：生命周期、超时清理、并发安全的会话注册表 | 竞态检测 `go test -race` 通过 |
| FR-B06 | 三条独立接收循环（Bidi / Uni / Datagram）解耦分发 | 按 `channel_id` + `stream_kind` 路由到对应处理器 |
| FR-B07 | ALF 帧编解码器 + 分片 / 重组器（含乱序容忍、超时丢弃、校验和） | 表驱动单测覆盖畸形 / 截断 / 越界 / 超长输入 |
| FR-B08 | 四类逻辑通道：`video`(Uni) / `audio`(Datagram) / `cursor`(Datagram) / `signal`(Bidi) / `file`(Bidi 可靠) | 通道语义与 §4.4 优先级表一致 |
| FR-B09 | 文件可靠传输：分块、进度、端到端 SHA-256 校验 | 30% 丢包下校验必须一致（见 §6 验收基线） |

### 4.3 后端 —— 多路复用优先级调度器

| ID | 需求 | 验收要点 |
|---|---|---|
| FR-B10 | 高频读写循环运行于独立 Goroutine，per-session 生命周期绑定 `context` | 会话关闭后无 Goroutine 泄漏（`goleak` 断言） |
| FR-B11 | 分级优先队列：`signal` > `audio` > `video` > `file`，采用严格优先级 + 加权公平（WFQ）防饥饿 | 低优先级队列在高负载下仍有可测量的最小份额 |
| FR-B12 | 令牌桶速率整形，速率由 BBR 模块（FR-B13）输出驱动 | pacing 变化可在大屏观测到吞吐响应 |
| FR-B13 | 应用层 BBR v1 四状态机（`Startup`/`Drain`/`ProbeBW`/`ProbeRTT`），输入为真实 `ConnectionStats()` | 丢包率切换后状态转移可从时间线断言 |
| FR-B14 | 背压：队列满时按通道语义处置（可靠通道阻塞 / 不可靠通道丢弃并计数），**禁止无声吞掉** | 丢弃计数在大屏可见 |

### 4.4 通道与优先级矩阵（SSOT）

| 通道 | 传输原语 | 可靠性 | 优先级 | 典型载荷 | 丢弃策略 |
|---|---|---|---|---|---|
| `signal` | Bidi Stream | 可靠有序 | P0（最高） | 指标下推、netem 控制、房间事件 | 永不丢弃 |
| `audio` | Datagram | 不可靠 | P1 | 合成音调 / 麦克风 PCM 切片 | 超时即丢，计数 |
| `cursor` | Datagram | 不可靠 | P1 | 鼠标轨迹点（含时间戳） | 超时即丢，计数 |
| `video` | Uni Stream | 可靠无序（跨流） | P2 | 合成 / 摄像头帧（分片） | 队列满丢整帧，计数 |
| `file` | Bidi Stream | 可靠有序 | P3（最低） | 文件分块 | 永不丢弃，背压阻塞 |

### 4.5 后端 —— 网络损伤仿真（netem）

| ID | 需求 | 验收要点 |
|---|---|---|
| FR-B15 | `net.PacketConn` 装饰器，UDP 包粒度注入丢包 / 延迟 / 抖动 / 乱序，上下行独立 | QUIC 层 `PacketsLost` 随之真实上升 |
| FR-B16 | 运行时热切换预设 0 / 10% / 30% / 50%，经信令通道下发，无需重启 | 切换延迟 < 100 ms |
| FR-B17 | 可播种随机源，保证实验可复现 | 同 seed 同配置两次运行统计量偏差 < 5% |

### 4.6 前端 —— 弱网对抗测试大屏

| ID | 需求 | 验收要点 |
|---|---|---|
| FR-F01 | 能力探测：`window.WebTransport` 缺失时展示友好降级页（自定义 Modal，**禁止原生 alert**） | 覆盖 Safari ≤26.3 / iOS ≤26.3 / 内置 WebView |
| FR-F02 | 连接编排：先 fetch 证书指纹 → 构造 WebTransport（`algorithm` 硬编码小写 `"sha-256"`，`allowPooling` 不传） | 连接失败给出可读原因，非静默 |
| FR-F03 | 三张实时曲线：RTT（双源对比）、吞吐量（上下行）、帧率 | 刷新 ≥ 10 Hz；滑动窗口 1000 点不掉帧 |
| FR-F04 | 丢包率切换控件（0 / 10% / 30% / 50%），切换瞬间在曲线上打时间标记 | 标记与后端切换时刻对齐 |
| FR-F05 | BBR 状态时间线可视化（四状态色带 + `BtlBw` / `RTprop` / `pacing_gain` 读数） | 状态与后端上报一致 |
| FR-F06 | 连接与会话诊断面板：QUIC 版本、GSO、Datagram 支持、`maxDatagramSize`、会话时长 | 值来自真实 API，非硬编码 |

### 4.7 前端 —— 多通道数据画布

| ID | 需求 | 验收要点 |
|---|---|---|
| FR-F07 | 文件上传经可靠通道，展示进度与端到端 SHA-256 校验结果 | 弱网下仍显示"校验一致" |
| FR-F08 | 鼠标轨迹经 Datagram 实时回显于 Canvas，**并排**渲染"本地真实轨迹"与"经网络回环轨迹" | 丢包下可肉眼看到回环轨迹断点 |
| FR-F09 | 音频经 Datagram 传输，展示波形 / 电平与丢帧计数 | 丢帧计数与后端统计一致 |
| FR-F10 | 视频源双模式切换（合成信号源 / `getUserMedia`），默认合成源 | 无摄像头环境可完整演示 |
| FR-F11 | 通道流量总览：各通道实时速率、队列深度、丢弃计数 | 与后端调度器指标一致 |

---

## 5. 非功能需求（NFR）

| ID | 需求 |
|---|---|
| NFR-01 | **Docker 交付标准**：`docker compose up --build -d` 一键启动，无手工步骤；`localhost` 可访问 Web 界面 |
| NFR-02 | **跨平台**：基础镜像同时支持 `linux/arm64` 与 `linux/amd64` |
| NFR-03 | **美学卓越**：Vue 3 + TypeScript + Tailwind，深色控制台风格；响应式覆盖 768px 与 480px 断点；页面级容器 `w-full` 不硬性限宽 |
| NFR-04 | **统一 Logger**：结构化日志（`slog`），level 可配；**禁止散落 `fmt.Println` / `console.log`**，生产环境屏蔽 debug |
| NFR-05 | **统一错误处理**：错误提示支持手动关闭（×）与 5s 自动消失；危险操作需自定义确认弹窗；**禁止原生 alert / confirm / prompt** |
| NFR-06 | **无死按钮**：所有交互入口要么实现功能，要么移除 |
| NFR-07 | **测试覆盖**：ALF 编解码 / 分片重组、netem、BBR 状态机、优先级调度器、会话管理必须有单元测试；`go test -race` 通过；E2E 用 Playwright 驱动 Chromium 实连 |
| NFR-08 | **API 文档**：独立 `docs/API.md`，含每端点请求 / 响应示例、参数类型、错误码表，以及 ALF 帧二进制布局图与信令消息 schema |
| NFR-09 | **配置外置**：端口、netem 预设、origin 白名单、优先级权重、BBR 参数全部环境变量 / 配置文件驱动，**禁止硬编码** |
| NFR-10 | **时区**：容器 `TZ=Asia/Shanghai`；用户可见时间统一 `yyyy-MM-dd HH:mm:ss` |
| NFR-11 | **优雅关闭**：`SIGTERM` 后停止接受新会话，通知在连会话，超时强制关闭；无 Goroutine 泄漏 |
| NFR-12 | **窄重试**：仅对瞬态网络错误重试；证书校验失败 / origin 拒绝 / 协议违规**一律不重试** |

---

## 6. 可量化验收基线（Phase 4 / Phase 5 断言依据）

> SOP v13 要求：有行业基准的维度必须写成可测量条件，不得用散文描述。

| # | 指标 | 基线 | 测量方式 |
|---|---|---|---|
| A-01 | 空载（0% 丢包）本机 `SmoothedRTT` p50 | **< 5 ms** | 服务端 `ConnectionStats()` 采样 60s |
| A-02 | **30% 丢包下可靠通道文件传输完整性** | **零字节错误**（SHA-256 完全一致） | 传 1 MB 与 10 MB 各一次，比对摘要 |
| A-03 | 50% 丢包下信令通道连接保持 | **≥ 60s 不断连** | keep-alive 生效，会话状态持续 active |
| A-04 | 30% 丢包下不可靠通道应用层丢帧率 | **≈ 注入丢包率 ±5%** | 证明不重传语义正确；显著低于则说明"不可靠"通道被误实现为可靠 |
| A-05 | 大屏指标刷新率 | **≥ 10 Hz**，UI ≥ 30 fps | 1000 点滑动窗口下不掉帧 |
| A-06 | BBR 状态收敛 | 丢包率切换后 **≤ 3 个 `RTprop`** 内完成 `ProbeBW → Drain` 转移 | 从 BBR 状态时间线断言 |
| A-07 | netem 可复现性 | 同 seed 同配置两次运行统计量偏差 **< 5%** | 两轮对比 |
| A-08 | 单机负载 | 500 datagram/s/session 下单核 CPU **< 50%** | 容器内 `docker stats` |
| A-09 | 启动时间 | `docker compose up` 到 `/api/v1/health` 返回 healthy **< 30s**（不含镜像构建） | 冒烟脚本计时 |
| A-10 | QA 每轮花费 | **¥0** | 无计费外部 API；测试全程 Mock / 离线 |

---

## 7. 明确不做（Out of Scope）

以下项**明确不在交付范围**，Phase 5 Auditor **不得**以其缺失判 FAIL：

- 完整 SFU 能力：simulcast 层切换、SVC、带宽估计驱动的订阅降级
- 媒体编解码 / 转码（不引入 ffmpeg、不做 H.264 / Opus 编码）
- 录制、点播、存储归档
- WebRTC 互通、TURN / ICE / SDP 协商
- 修改 quic-go 传输层内核拥塞控制（理由见 C-01）
- 从零重写 WebTransport 协议栈（理由见 C-02）
- 用户账号体系、鉴权与权限（实验台按单租户设计；提供房间 ID 隔离，不提供身份认证）
- 公网部署、真实域名与公信 CA 证书签发
- 水平扩展 / 集群 / 会话跨节点迁移

---

## 8. 技术栈与交付形态

### 8.1 技术栈

| 层 | 选型 | 理由 |
|---|---|---|
| 后端语言 | Go 1.25（实测宿主 `go1.25.12`） | 需求指定 |
| QUIC / H3 | `quic-go v0.61.0` | 需求指定，版本已锁定实测 |
| WebTransport | `webtransport-go v0.12.0` | 官方配套，避免重写协议栈（C-02） |
| 日志 | 标准库 `log/slog` | 零依赖结构化日志 |
| 前端 | Vue 3 + TypeScript + Vite + Pinia | 需求指定 Vue 3 |
| 样式 | Tailwind CSS | Redline 2 认可的现代设计系统 |
| 图表 | 轻量 Canvas 自绘或 uPlot 级库 | 10 Hz × 1000 点需要高性能，避免重型图表库掉帧 |
| E2E | Playwright（Chromium） | 需真实浏览器验证 WebTransport 与证书 pin |
| 容器 | Docker 多阶段构建 + Compose | Redline 1 |

### 8.2 目录结构（遵循 SOP 严格结构）

```
backend/            # Go 服务（约 35–45 个 .go 文件）
frontend-user/      # Vue 3 控制台
docs/               # SSOT 文档
tests/              # E2E 与冒烟
docker-compose.yml
```

后端模块划分建议（Phase 1 Architect 细化）：
`cmd/server` · `internal/transport`(QUIC/H3/WT 装配) · `internal/alf`(帧协议) · `internal/netem`(损伤仿真) · `internal/congestion/bbr`(应用层 BBR) · `internal/scheduler`(优先级/WFQ/令牌桶) · `internal/session` · `internal/room` · `internal/metrics` · `internal/signal` · `internal/certs` · `internal/config` · `internal/logging` · `internal/httpapi`

### 8.3 端口规划

| 阶段 | TCP（页面 + REST） | UDP（H3 / WebTransport） | 说明 |
|---|---|---|---|
| Dev | `19443` | `19444` | 已实测空闲；避开 devops 记忆提示的 `1848x` 冲突段 |
| Deploy（`/deploy` 阶段改写） | `8081` | `8082` | SOP 标准 8081+ |

约束：容器内外端口号 **1:1 映射**；UDP 必须显式声明 `/udp`；页面走 plain HTTP（`localhost` 即 secure context，绕开 TLS 警告）；origin 白名单须同时含 `localhost` 与 `127.0.0.1`。

---

## 9. 风险登记

| # | 风险 | 等级 | 缓解 |
|---|---|---|---|
| R-01 | Docker Desktop for Mac 的 UDP 转发吞掉大 QUIC 包 | **高** | Phase 4 首个冒烟即验证 1 MB 流传输；备选降低 `InitialPacketSize` 或关闭 PMTUD |
| R-02 | 目标 Chrome 拒绝自签证书 pin | **高** | 严格遵守 C-03 六条；Playwright 断言 `transport.ready` resolve；备选 `--webtransport-developer-mode` |
| R-03 | 应用层 BBR 被判"伪实现" | **中高** | 输入必须真实、输出必须接线、README §7 必须披露层次；提供 BBR 开 / 关吞吐对比实验 |
| R-04 | 代码量突破 9000 行上限 | **中** | 严守 §7 Out of Scope；Roadmap 划 MVP / V1 边界 |
| R-05 | 模型按旧 `logging.ConnectionTracer` API 生成代码导致编译失败 | **中** | `api_contracts.md` §2.4 已警示；遥测统一走 `ConnectionStats()` |
| R-06 | 10 Hz × 1000 点前端掉帧 | **中** | 避免重型图表库；`requestAnimationFrame` 批量重绘 + 环形缓冲，禁止响应式深监听大数组 |
| R-07 | 高频 Goroutine 泄漏 | **中** | per-session `context` 严格绑定；`goleak` 纳入单测 |

---

## 10. 冻结声明

本文档为本项目 **WHAT** 的唯一权威来源（SSOT）。

- 所有技术结论均有 `docs/.meta/api_contracts.md` 中的实测证据支撑，非推测。
- 与用户原始需求存在偏离的 9 处（C-01 至 C-09）已逐条登记理由与处置方案，其中 **C-01 与 C-02 涉及对原需求措辞的重新解释，必须在 `README.md` §7 向用户显式披露**。
- 后续 Phase 1 产出的 `docs/Roadmap.md` 定义 **WHEN**，不得扩大本文档的 WHAT。

**需求已冻结（Requirements Frozen）。**
