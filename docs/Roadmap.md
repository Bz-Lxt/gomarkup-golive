# 实施路线图

> 项目：GoLive — HTTP/3 / WebTransport 实时流媒体控制台
> 权威 WHAT：`docs/Requirements.md`（已冻结）
> 本文档定义 WHEN。不得扩大 WHAT。
> 冻结时间：2026-08-23 (GMT+8)

---

## 0. 阶段顺序决策（v13 强制）

**决定：Logic-First（交换 Phase 2 与 Phase 3）。**

**理由一句话**：前端大屏的组件树、曲线轴、通道画布与 BBR 色带完全派生于 ALF 帧布局、信令 JSON schema 与 `ConnectionStats` 下推结构；schema 未定之前画 UI 必然返工。

执行顺序：

1. Phase 1  Architect — 结构 / compose / 协议契约
2. Phase 3  Logic     — Go 后端 + Docker 接线（本文件标记为「逻辑阶段」）
3. Phase 2  UI        — DesignSpec + Vue 3 控制台对接真实 schema
4. Phase 4  QA        — Mock / 离线，¥0
5. Phase 5  Auditor   — 对照 `audit-rules.md` + Knowledge Harvest

---

## 1. 范围分层

| 层 | 内容 | 边界 |
|---|---|---|
| **MVP** | QUIC/H3/WT 握手、ALF 编解码、netem 热切换、双源指标下推、文件可靠传输、鼠标 Datagram 回环、合成音视频、应用层 BBR + 优先级调度、Vue 大屏 | 单房间回环 + 广播 |
| **V1（本轮交付）** | MVP 全部 + 证书自动签发/重签、origin 白名单、goleak、Playwright 实连、`docs/API.md` | 代码量 6500–9000、≥35 个 `.go` |
| **V2（不做）** | SFU simulcast / SVC、ffmpeg 转码、账号体系、公网 CA、集群 | 见 Requirements §7 |

---

## 2. 模块与文件预算

| 包 | 职责 | 预估文件 | 预估行数 |
|---|---|---|---|
| `internal/clock` | GMT+8 单调时钟 | 2 | 80 |
| `internal/logging` | slog 统一 Logger | 1 | 60 |
| `internal/config` | 环境变量驱动配置 | 2 | 220 |
| `internal/certs` | ECDSA P-256 / 13 天 / 指纹 | 2 | 260 |
| `internal/alf` | 帧协议 / 分片 / 重组 | 5 | 900 |
| `internal/netem` | PacketConn 损伤仿真 | 4 | 550 |
| `internal/congestion/bbr` | 应用层 BBR v1 | 4 | 700 |
| `internal/scheduler` | 优先级 / WFQ / 令牌桶 | 5 | 650 |
| `internal/metrics` | ConnectionStats 采集 | 3 | 350 |
| `internal/signal` | 信令消息 schema | 2 | 280 |
| `internal/session` | 会话 / 三循环 / 文件 | 5 | 900 |
| `internal/room` | echo / fan-out | 2 | 250 |
| `internal/httpapi` | TCP REST | 3 | 350 |
| `internal/transport` | H3 + WT + netem 装配 | 3 | 450 |
| `internal/app` + `cmd/server` | 组装 / 优雅关闭 | 2 | 280 |
| **合计** | | **≈ 45** | **≈ 6300–7000**（含测试可达 8000+） |

---

## 3. 任务清单

### 逻辑阶段（原 Phase 3）

- [x] L-01 配置 / 时钟 / 日志 / 证书
- [x] L-02 ALF 编解码 + 表驱动单测
- [x] L-03 分片重组器（乱序 / 超时 / 校验）
- [x] L-04 netem PacketConn（seeded，上下行独立）
- [x] L-05 应用层 BBR v1 四状态机
- [x] L-06 调度器：严格优先级 + WFQ + 令牌桶
- [x] L-07 会话三循环（Bidi / Uni / Datagram）
- [x] L-08 文件可靠传输 + SHA-256
- [x] L-09 房间 echo / fan-out
- [x] L-10 HTTP/3 + WebTransport 装配（自定义 PacketConn）
- [x] L-11 TCP REST：health / fingerprint / netem / config
- [x] L-12 Dockerfile 多阶段 + compose（端口 19443/19444）

### UI 阶段（原 Phase 2）

- [x] U-01 `docs/DesignSpec.md`（雷达控制台美学）
- [x] U-02 连接编排（先指纹后 WT）
- [x] U-03 三曲线 + BBR 时间线
- [x] U-04 多通道画布（文件 / 鼠标 / 音频 / 视频）
- [x] U-05 能力探测降级页 + 统一 Toast / Modal

### QA / Audit

- [x] Q-01 `go test -race` 核心包
- [x] Q-02 `tests/api_smoke.py` + 浏览器实连（Mock / 离线 ¥0）
- [x] A-01 `docs/AuditReport.md` + `/learn`

---

## 4. 端口（Dev）

| 服务 | 协议 | 宿主 | 容器 | 已探测 |
|---|---|---|---|---|
| 页面 + REST | TCP | 19443 | 19443 | 空闲 |
| H3 / WebTransport | UDP | 19444 | 19444 | 空闲 |

`/deploy` 阶段改写为 8081 / 8082。容器内外 1:1，UDP 显式 `/udp`。

---

## 5. 协议契约（前端依赖，先于 UI）

见 `docs/API.md`（逻辑阶段同步产出）。核心不变量：

- ALF header 固定 35 字节；Datagram 分片预算 `min(maxDatagram, 1024) - 35`
- `algorithm` 必须小写 `"sha-256"`
- 指标源 A：`ConnectionStats` 10 Hz；源 B：Datagram ping/pong
- `PacketsLost` 非单调，前端画瞬时速率
