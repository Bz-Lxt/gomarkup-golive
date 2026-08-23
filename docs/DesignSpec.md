# Design Spec — GoLive Console

> 美学方向：**深海雷达控制台**（industrial / phosphor），不是通用仪表盘。
> 受众：网络工程师在弱网实验中同时盯三条曲线、一条 BBR 色带、一块多通道画布。

## 1. 概念

界面应让人觉得自己坐在一台正在扫描路径质量的仪器前：深海军底、磷光描边、等宽读数、细扫描线。每一个数字都要像示波器读数一样可溯源，而不是卡片里的 KPI。

## 2. 色彩

| Token | Hex | 用途 |
|---|---|---|
| `--bg` | `#070b10` | 页面底 |
| `--panel` | `#0e1620` | 面板 |
| `--panel-2` | `#121c28` | 嵌套槽 |
| `--line` | `#1c2c3c` | 分割 |
| `--cyan` | `#3ee0c5` | 主强调 / 吞吐 |
| `--amber` | `#f0b429` | 丢包 / 告警 |
| `--red` | `#ff4d6d` | 危险 / 50% 档 |
| `--green` | `#8cff6b` | 应用层 RTT / 成功 |
| `--violet` | `#9b8cff` | BBR ProbeBW |
| `--ink` | `#d7e4ef` | 正文 |
| `--mute` | `#7b8b99` | 次级 |

BBR 状态色带：Startup=`--amber` Drain=`--red` ProbeBW=`--violet` ProbeRTT=`--cyan`。

## 3. 字体

- 展示：`Oxanium`（几何、仪表感）
- 读数：`Share Tech Mono`（磷光等宽）
- 禁止 Inter / Roboto / Arial / Space Grotesk

## 4. 布局

全宽 `w-full`（非 Auth 卡片）。三行：

1. 顶栏：品牌、连接状态、证书剩余小时、QUIC 版本、GSO、Datagram
2. 中部 12 列：左 8 列三曲线 + BBR 色带；右 4 列丢包档位 / 会话诊断 / 通道队列
3. 底部：文件传输 | 鼠标双轨迹画布 | 音频电平 | 视频预览

断点：768px 单列堆叠；480px 顶栏压缩、画布全宽。

## 5. 交互

- 禁止原生 `alert` / `confirm` / `prompt`
- Toast：可 × 关闭，5s 自动消失
- 危险操作（断开、清空画布）走自定义 Modal
- 无死按钮：不支持 WebTransport 时整页降级，不留灰按钮
- 曲线 10 Hz 刷新，环形缓冲 1000 点，`requestAnimationFrame` 批绘，禁止深度 watch 大数组

## 6. 成本

本项目无计费 API。控件不展示费用。
