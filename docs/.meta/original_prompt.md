# 原始需求（用户原文，逐字保存，禁止改写）

> 记录时间：2026-08-23 00:44 (GMT+8)
> 记录人：PM Agent (Alkaid-SOP v13.0)
> 用途：Phase 5 Auditor Agent 的评判基准输入。任何后续文档与本文件冲突时，以本文件为准。

---

帮我用Go语言实现一款现代 HTTP/3 与 WebTransport 实时流媒体控制台（Mini LiveKit / 新代网络网关）

业务背景：基于下一代互联网协议 QUIC (HTTP/3)，实现一个极低延迟、抗网络抖动的实时音视频/大块数据传输全栈控制台。

前端页面 (Vue 3 + WebTransport API)：弱网对抗测试大屏：前端利用 WebTransport 连接后端，实时展示在模拟丢包率（10%、30%、50%）下的网络延迟（RTT）、吞吐量和帧率曲线。多通道数据画布：支持通过可靠通道（Stream）传输文件，通过不可靠通道（Datagram）传输鼠标轨迹或音频，并在画布上实时呈现。

Go 后端核心实现：QUIC/HTTP3 原生服务器：利用 quic-go 库，深度定制底层连接的拥塞控制算法适配（如 BBR 策略微调）。WebTransport 协议握手与解包：完整实现 WebTransport 帧格式的解析，将双向流（Bidi-Streams）和单向流（Uni-Streams）解耦分发。多路复用调度器：在 Goroutine 中实现高频的读写循环，对视频、音频、信令设置不同的优先级队列。

文件数与代码量：约 35+ 个 Go 文件，代码量 6500 - 9000 行。
