# GoLive

HTTP/3 + WebTransport 实时传输实验台（Mini LiveKit 的网关切片）。

## 1. 如何启动

```bash
docker compose up --build -d
```

打开 http://localhost:19443 。无需手工签发证书、无需安装浏览器插件。

环境变量见 `docker-compose.yml`。时区 `TZ=Asia/Shanghai`。UDP 端口必须映射为 `19444:19444/udp`。容器默认 `QUIC_GO_DISABLE_GSO=1`。

## 2. 使用说明

点击「连接」。前端会先拉取 `/api/v1/cert-fingerprint`，再用小写 `sha-256` 的 leaf DER 指纹 pin 到 `https://localhost:19444/webtransport`。

左侧三条曲线分别是 RTT（传输层 `ConnectionStats` vs 应用层 ping/pong）、吞吐、帧率。右侧切换 0/10/30/50 丢包，损伤作用在 UDP 包层，QUIC 会真实重传。底部：可靠通道传文件并校验 SHA-256；Datagram 画鼠标回环；合成或摄像头媒体源。

不支持 WebTransport 的浏览器会看到降级页，没有灰掉的死按钮。

## 3. 服务列表及API说明

| 入口 | 地址 |
|---|---|
| 控制台 + REST | http://localhost:19443 |
| Health | http://localhost:19443/api/v1/health |
| 证书指纹 | http://localhost:19443/api/v1/cert-fingerprint |
| WebTransport | https://localhost:19444/webtransport |

完整契约见 `docs/API.md`。

## 4. 测试账号

无账号体系。单租户实验台，房间 ID 默认 `default`。

## 5. 题目内容

用 Go 实现现代 HTTP/3 与 WebTransport 实时流媒体控制台：弱网对抗大屏、可靠/不可靠多通道画布、QUIC 服务器、WebTransport 流解耦、多路复用优先级调度。详见 `docs/.meta/original_prompt.md`。

## 6. 项目结构

```
backend/           Go 服务（quic-go v0.61.0 + webtransport-go v0.12.0）
frontend-user/     Vue 3 控制台
docs/              Requirements / Roadmap / API / DesignSpec
tests/             api_smoke.py / e2e_flow.spec.ts
docker-compose.yml
```

## 7. API 模拟与切换指南

本项目**没有计费外部 API**。两条必须说清楚的「模拟」：

**1. 应用层 BBR（不是传输层内核）**  
`quic-go v0.61.0` 的公开 API 不暴露拥塞控制，`internal/congestion` 只有 Cubic。BBR v1 四状态机跑在调度器令牌桶上，输入是真实 `Conn.ConnectionStats()`，输出真实限制出队速率。开关：环境变量 `BBR_ENABLED=true|false`，或信令 `bbr.set`。大屏复选框可热切换。隐瞒「应用层而非 quic-go 内核」即伪实现。

**2. 媒体源**  
默认合成 Canvas/WebAudio 信号源（容器无摄像头）。按钮「摄像头」走真实 `getUserMedia`；失败自动回退合成源。后端不做转码。

**不是 mock 的部分**：netem 包装 `net.PacketConn` 在 UDP 粒度丢包/延迟/抖动，QUIC 丢包计数会真实上升；RTT/吞吐来自 `ConnectionStats()`；文件校验是端到端 SHA-256。
