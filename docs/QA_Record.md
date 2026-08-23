# QA Record

## Round 1 · 2026-08-23 17:02 (GMT+8)

**Cost**: ¥0（全程 TCP REST + 本机 WebTransport，无计费 API）

### 环境

- `docker compose up --build -d` 成功；服务 `golive-golive-1` healthy
- 宿主 `http://127.0.0.1:19443` / UDP `19444`

### 结果

| 项 | 结果 | 证据 |
|---|---|---|
| Docker Build | PASS | 镜像 `golive:dev` 构建完成并启动 |
| Health Check | PASS | `{"status":"ok","udp_listening":true,"cert_hours_remaining":311,"bbr_layer":"application-scheduler","time":"2026-08-23 17:01:40"}` |
| 证书约束 | PASS | algorithm=`sha-256`，P-256，有效期 13 天（08-23 → 09-05） |
| REST 冒烟 `tests/api_smoke.py` | PASS | 10/10，含 422 拒未知预设 |
| WebTransport 握手 | PASS | 浏览器 `sid c498f5cc`，`dgram yes`，`QUIC 1` |
| 指标下推 | PASS | 队列可见，BBR pacing/BtlBw 有读数，合成源 fps≈10 |
| netem 热切换 | PASS | UI 标记 `30%`；`GET /api/v1/netem` uplink.loss_pct=30 |
| `go test -race` 核心包 | PASS | alf/netem/bbr/scheduler/certs/config/clock/room/signal/metrics/httpapi/transport |

### 本轮修复（未翻案）

1. 前端 `readExact` 丢弃同一 chunk 剩余字节 → `BytePump` 保留 leftover
2. 服务端复用 signal bidi 后客户端只读一帧 → `iterateFrames` 持续读
3. `r.Context()` 取不到 `quic.Conn` → ConnContext 登记 + RemoteAddr 回退

### Playwright E2E

未在 CI 容器内安装 Playwright。等价验证已用 Cursor 浏览器走完：连接 → welcome/sid → 指标 → 30% 热切换。`tests/e2e_flow.spec.ts` 保留给后续本机 `npx playwright test`。
