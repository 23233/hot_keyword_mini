# 爆款热词与短剧小程序矩阵一体化系统 (SDUI 动态组件引擎)

本项目为针对微信搜一搜爆款关键词与上升指数词流量承接打造的高并发小程序矩阵系统。采用 **Golang (Iris + Gorm) 服务端驱动动态引擎 (Server-Driven UI, SDUI)** 与 **Taro 4 + React 微信小程序跨端技术栈**，界面严格遵循**苹果人机交互设计规范 (Apple HIG)**，并配备了**多租户一脑多控管理工作台与微信开发者工具默认 iPhone 12/13 Pro 同构模拟器**。

---

## 🌟 核心架构与核心能力

### 1. 服务端驱动动态组件引擎 (SDUI Engine)
- **零发版敏捷上新**：页面布局、原子积木、主题样式（暗夜磨砂/极简冷白/赛博霓虹）、强调色与点击行为完全由后端 JSON 协议驱动，发布后线上小程序 0ms 即时热更新，彻底摆脱小程序审核周期；
- **苹果 HIG 全行业原子积木库**：
  - `MediaHeroBlock`：媒体大焦点海报、高清试看视频、微信视频号原生免跳播放与跳转；
  - `ResourceCardBlock`：多网盘渠道（夸克、百度网盘）提取码卡片与一键复制；
  - `GameCardBlock`：游戏公测介绍、独家礼包兑换码与一键复制启动；
  - `FormBlock`：考分/物流/证件受控查询输入表单；
  - `ActionButtonBlock`：苹果高光大胶囊主操作按钮；
  - `NoticeBlock`：跑马灯通告栏；
  - **优雅降级调度器 (`BlockRenderer`)**：具备组件版本检查与降级策略，遇到未知新积木优先读取 fallback 块或优雅占位，杜绝全页白屏崩溃；
- **万能动态承载页 (`pages/dynamic/index`)**：
  - 动态导航栏标题与毛玻璃吸顶；
  - 骨架屏秒开体验与空错态优雅回退；
  - 微信好友分享与朋友圈分享全生命周期动态绑定。

### 2. 企业级多租户矩阵与双 Token 会话底层
- **多租户数据物理隔离**：
  - 小程序前端不内置 AppID，请求由微信运行时 `Referer` 或 `X-WX-AppID` 识别租户；
  - 后端中间件提取租户上下文，数据库表（`users`, `dynamic_pages`, `user_sessions`）均以 `app_id` 为联合唯一键隔离；
- **双 Token 轮换与防重放攻击 (Anti-Replay Attack)**：
  - 短期 Access Token（2小时有效）+ 长期加密 Refresh Token（30天有效）；
  - 每次刷新令牌实行旧 Token 吊销轮换；一旦监测到已轮换 Token 再次被使用，判定为凭证泄露或重放攻击，立即强行撤销该会话族所有登录态；
  - 前端具备并发刷新单例共享锁（Promise Lock），防止多接口同时 401 发生请求风暴。

### 3. 多租户一脑多控可视化工作台 (`/admin`)
访问地址：`http://localhost:8080/admin`
- **多小程序一脑多控切换器**：
  - 顶部下拉框自由切换任意小程序主体，支持“+ 新建小程序”录入新 AppID 与密钥，单点后台统一操盘几十款小程序；
- **四大指数词行业爆款模板包库**：
  - **短剧爆款承接模板 (`tpl_drama_standard`)**：试看视频 + 网盘多渠道 + 公众号防走丢；
  - **游戏礼包兑换码模板 (`tpl_game_redeem`)**：公测通告 + 独家礼包兑换码 + 启动游戏；
  - **通用信息查询结果模板 (`tpl_query_result`)**：考分查询表单 + 查询结果时间线；
  - **极速软件/资源下载模板 (`tpl_download_resource`)**：安装包说明 + 夸克/百度网盘高速通道；
  - 点击一键套用模板瞬间派生当前页面，生成后支持逐块自由增删改查；
- **iPhone 12/13 Pro 手机模拟器 0ms 实时同构渲染**：
  - 左侧模拟器采用真实 CSS 毛玻璃磨砂与苹果质感，与小程序同构渲染；
  - 右侧改动任何标题、积木内容、样式或动作，左侧模拟器 0ms 实时热更新，所见即所得。

### 4. 后端无头截屏与微信分享卡片生成服务
- **纯 Go 原生图层渲染引擎**：
  - 零外部 Chromium / Puppeteer 沉重依赖，全平台跨平台支持，内存开销 <15MB，合成速度 <30ms；
  - 微信聊天分享卡片：**官方 5:4 比例 (1000 × 800)**；
  - 微信朋友圈分享图：**官方 1:1 比例 (800 × 800)**；
- **全链路自动配图闭环**：
  - 公开端点：`GET /api/v1/share/card?app_id=...&page_id=...&type=app_message` 以 `image/png` 直接输出；
  - 管理后台一键“⚡ 自动生成微信 5:4 分享卡片”，自动回写并持久化至页面分享配置，转发时微信客户端自动展现高清卡片。

### 5. AI Model Context Protocol (MCP) 编排服务与 7 大受控工具
遵循 Model Context Protocol (JSON-RPC 2.0) 规范，支持外部 AI Agent 自主完成受控编排闭环：
- **`sdui.template.list`**：查询短剧/游戏/查询/下载四大行业可用模板；
- **`sdui.page.create`**：从模板派生创建草稿页面 (`status: draft`)；
- **`sdui.page.patch`**：受控局部 JSON Patch 补丁原子打补丁；
- **`sdui.page.validate`**：语法、ID冲突与动作参数机器可读强校验报告；
- **`sdui.page.preview`**：模拟装配并输出响应信封；
- **`sdui.page.screenshot`**：规范化图层合成，输出卡片 URL 与 SHA-256 图像哈希；
- **`sdui.page.publish`**：通过强校验后显式确认发布，沉淀不可篡改版本快照；
- **双通道接入**：支持 HTTP 端点 `POST /api/v1/mcp` 与独立 Stdio 命令行服务 `go run ./cmd/mcp-server`。

---

## 📁 系统工程目录结构

```
├── cmd/
│   └── mcp-server/             # 独立 Stdio 传输协议 AI MCP 命令行服务 (main.go)
├── models/                     # GORM 数据模型层
│   ├── miniapp.go              # 多租户小程序模型 (MiniApp)
│   ├── sdui.go                 # SDUI 动态页面 (DynamicPage)、原子积木 (BlockItem)、动作 (BlockAction)、分享 (PageShareConfig)
│   ├── session.go              # 用户持久化会话与防重放模型 (UserSession)
│   ├── drama.go                # 短剧主表 (Drama)、选集表 (DramaEpisode)、页面配置 (PageConfig)
│   ├── user.go                 # 用户模型 (联合唯一索引隔离 AppID)
│   └── comm.go                 # 自定义数据字段
├── services/                   # 后端业务逻辑层 (遵循工业级规范，逻辑解耦)
│   ├── sdui.go                 # SDUI 页面编排组装、多租户页面管理、信封封装与 ETag 304 缓存
│   ├── template.go             # 行业模板包注册中心 (TemplateRegistry) 与页面派生
│   ├── share_card.go           # 微信分享卡片纯 Go 图层渲染与自动配图服务
│   ├── auth.go                 # 微信登录、双 Token 轮换状态机与重放攻击拦截
│   ├── drama.go                # 短剧业务聚合与种子数据自举
│   └── *_test.go               # 全套后端业务自动化单元测试
├── routers/                    # 路由与控制器层
│   ├── middleware/             # 租户拦截中间件 (TenantMiddleware)
│   ├── sdui.go                 # /api/v1/page/:page_id (SDUI 动态页面信封接口)
│   ├── auth.go                 # /api/v1/auth/wechat-login, /refresh, /session, /logout
│   ├── admin.go                # /api/v1/admin/apps, /pages, /page, /templates, /generate_share_card
│   ├── drama.go                # /api/v1/drama/home, /api/v1/drama/detail
│   ├── api.go                  # API 路由总控与 /api/v1/share/card 公开图片流
│   └── index.go                # Web 路由与 /admin 工作台入口
├── templates/
│   └── admin.html              # 可视化管理工作台 (多租户操盘 + 模板库 + iPhone 12/13 Pro 手机模拟器)
├── system/
│   └── migrate.go              # 数据库自动迁移与多租户/SDUI 种子自举注入
├── minifront/                  # 微信小程序前端工程 (Taro 4 + React + TypeScript)
│   ├── src/
│   │   ├── components/SDUI/    # 苹果 HIG 原子积木库
│   │   │   ├── MediaHeroBlock.tsx     # 媒体大卡片 (视频播放/视频号原生内嵌与跳转)
│   │   │   ├── ResourceCardBlock.tsx  # 网盘多渠道提取卡片 (一键复制+震动)
│   │   │   ├── GameCardBlock.tsx      # 游戏礼包卡片 (兑换码展示与复制)
│   │   │   ├── FormBlock.tsx          # 查询与输入表单卡片
│   │   │   ├── ActionButtonBlock.tsx  # 通栏大胶囊主操作按钮
│   │   │   ├── NoticeBlock.tsx        # 跑马灯通告栏
│   │   │   ├── BlockRenderer.tsx      # 积木动态调度器 (具备未知块优雅降级保护)
│   │   │   └── sdui.scss              # Apple HIG 磨砂高斯模糊规范样式表
│   │   ├── pages/dynamic/      # 万能 SDUI 动态承载页容器 (动态标题/骨架屏/分享闭环)
│   │   ├── utils/
│   │   │   ├── auth.ts         # 双 Token 会话状态机 (提前90s预刷新、401并发刷新共享锁)
│   │   │   ├── action.ts       # 万能动作分发器 (剪贴板震动、视频号拉起、跨小程序跳转、鉴权拦截)
│   │   │   └── request.ts      # 企业级请求层 (租户Header自动注入、401重试防死循环)
│   │   └── types/sdui.ts       # 前端 SDUI 协议 TypeScript 类型定义 (双端对齐)
│   ├── project.config.json     # 微信小程序配置文件
│   └── package.json            # 前端依赖管理
└── config.yaml                 # 系统配置文件
```

---

## 🚀 快速启动指南

### 1. 运行后端服务
```bash
# 运行单元测试
go test -v ./services/...

# 编译项目
go build ./...

# 启动 Iris 后端 (默认监听 http://localhost:8080)
go run main.go
```

### 2. 体验可视化管理后台
启动后端后，浏览器直接访问：
👉 **`http://localhost:8080/admin`**
1. **多小程序管理**：顶部切换或添加小程序；
2. **行业模板套用**：点击“📋 套用行业模板”，在短剧爆款、游戏礼包、考分查询、资源下载中自由选择；
3. **积木编排**：拖拽排序、增删组件、修改文字、配置跳转动作，左侧 iPhone 12/13 Pro 模拟器实时响应；
4. **一键生成分享卡片**：点击“⚡ 自动生成微信 5:4 分享卡片”，即刻实时预览并在发布后下发；
5. **发布生效**：点击右上角“⚡ 保存并同步至小程序”，线上小程序无需重新发布代码包即可秒级生效！

### 3. 小程序前端编译与调试
```bash
# 进入小程序目录
cd minifront

# 安装依赖
pnpm install

# 编译微信小程序产物
pnpm run build:weapp

# 使用微信开发者工具 CLI 打开预览
& "C:\Program Files (x86)\Tencent\微信web开发者工具\cli.bat" open --project "c:\Users\Admin\Desktop\projects\hot_keyword_mini\minifront"
```

### 4. 微信支付配置（每个小程序独立普通商户）

在管理后台为对应 AppID 填写 `payment_mch_id`、`payment_mch_serial_no`、`payment_api_v3_key`、`payment_private_key`。支付回调地址由后端根据 `public_base_url` 和 AppID 自动生成，不再在后台手工填写。其中 `payment_private_key` 是商户 API 私钥 `apiclient_key.pem` 的完整 PEM 内容，不是商户证书文件；`payment_mch_serial_no` 是商户 API 证书 `apiclient_cert.pem` 的证书序列号，平台证书由 SDK 使用 API v3 Key 自动下载和轮换。还必须在微信支付商户平台完成该小程序 AppID 与商户号绑定并开通 JSAPI/小程序支付权限。

商品通过商品表维护，前端动作只提交 SKU，金额始终由后端读取 `price_fen`。支付通知地址必须使用路径携带 AppID，例如 `https://实际域名/api/v1/payment/notify/wx2e8feeb13a20fb1b`；支付成功后客户端通过 `GET /api/v1/payment/orders/{out_trade_no}` 查询最终状态。
