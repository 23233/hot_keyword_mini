# 微信搜一搜爆款指数词敏捷流量收割系统：服务端驱动动态组件引擎 (SDUI) 架构设计

> **版本**：v1.3.0  
> **状态**：协议扩展设计（待评审）  
> **核心目标**：在已发布的小程序版本内，通过受控的 SDUI 协议快速切换指数词落地页、内容和转化路径；不承诺绕过平台审核，也不把任意代码下发到客户端。

---

## 一、系统定位与商业模型

### 1.1 核心痛点与商业背景
微信搜一搜的上升指数词（如热播短剧名、突发事件、考试成绩查询、爆款软件安装包、游戏兑换码等）具有**极强的时效性（爆发黄金期通常仅 1~3 天）**。
- **传统小程序痛点**：每次为了蹭新词修改页面排版和逻辑，都必须修改代码 -> 打包构建 -> 提交微信官方审核 -> 等待数小时至数天 -> 面临驳回拒审风险。往往审核通过时，该关键词的搜索流量峰值早已退去。
- **Webview 方案痛点**：体验差、卡顿白屏、需要配置业务域名、常被微信封禁域名、且无法深度调用微信原生组件（如视频号跳转、原生剪贴板等）。

### 1.2 破局之道：服务端驱动 UI (Server-Driven UI / SDUI)
打造一套运行在微信小程序原生环境下的**“动态原子组件编排引擎”**：
- 小程序端只预置**标准的原生苹果风原子积木库**和**万能动作执行器**；
- 页面显示什么结构、排版、样式，以及按钮点击做什么事，**100% 由后端 JSON 协议动态下发**；
- 管理后台提供所见即所得的 iPhone 16 Pro 手机模拟器，支持拼积木、配样式、绑定事件；
- **全网发布 0ms 生效**：发现热词 -> 后台改名改模版 -> 瞬间上线承接万级搜索流量！

---

## 二、顶层全景架构

```
┌────────────────────────────────────────────────────────────────────────┐
│                        可视化管理后台 (/admin)                         │
│  - iPhone 16 Pro 真实同构渲染模拟器 (所见即所得)                      │
│  - 多页面站点树状管理 (页面增删、设为首页)                              │
│  - 苹果 HIG 样式设计标记配置 (圆角/质感/间距/渐变)                     │
│  - 积木组件拖拽拼装与交互动作绑定                                      │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ 下发 UI 协议 JSON（已发布版本实时生效）
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                          Golang 后端服务端                             │
│  - 页面协议持久化存储 (MySQL + Redis 极速缓存)                         │
│  - 统一协议解析/绑定/校验中间表示 (IR)                                  │
│  - 模板库、MCP 编排服务与规范化截图服务                                │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ API: GET /api/v1/page/:page_id?id=...
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                     Taro React 小程序前端 (原生渲染)                   │
│                                                                        │
│  ┌────────────────────────────┐  ┌──────────────────────────────────┐  │
│  │    万能动态承载页容器      │  │     统一动作分发器 (Action)      │  │
│  │  - pages/index/index (主页)│  │  - 复制网盘/口令 (带震动Toast)   │  │
│  │  - pages/dynamic/index (子)│  │  - 原生直达视频号 (openChannels) │  │
│  └──────────────┬─────────────┘  │  - 多页面路由流转 (navigateTo)   │  │
│                 ▼                │  - 跳转其他小程序 (openMini)     │  │
│  ┌────────────────────────────┐  │  - 全屏大图预览 / 弹出抽屉       │  │
│  │  基础布局与业务块注册表    │  └──────────────────────────────────┘  │
│  │ (媒体/结果/资源/表单/按钮) │                                        │
│  └────────────────────────────┘                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### 2.1 同一内核、模板化行业能力

短剧、游戏、查询、下载、交易、预约等不是多套引擎，而是同一个 SDUI 内核上的行业模板包：

```text
SDUI 内核
  ├─ 协议解析、绑定、条件、状态、动作、分享、埋点、降级
  ├─ 基础布局与内容块
  └─ 行业模板包
      ├─ drama：短剧主页、选集、播放、全集承接
      ├─ game：游戏主页、游戏详情、兑换码、开服活动
      ├─ query：查询表单、结果页、状态时间线
      ├─ download：资源详情、版本选择、下载/复制
      └─ custom：按块逐个编排的自定义页面
```

行业模板包只提供页面结构、实体字段、允许块集合和常用动作的预设，不拥有独立渲染逻辑。后台或 AI 可以先选择模板快速生成页面，再对任意块的内容、顺序、条件、样式、请求和分享配置逐项修改；模板生成后的页面必须仍是标准 `DynamicPage`，不能产生模板专属私有协议。

模板包需要声明 `template_id`、`template_version`、适用的 `business_type`、所需实体、默认块树、可覆盖字段和兼容的客户端版本。模板升级不得覆盖已发布页面，只能生成新草稿或由管理员明确迁移。

---

## 三、统一数据协议规范 (Schema)

为保证 Golang 后端与 Taro React 前端的完美对齐与类型安全，制定双端镜像结构：

### 3.1 动作协议 (`BlockAction`)
定义任意按钮、卡片、图片被点击时的标准原子行为态（含跨小程序矩阵联动跳转）：
```json
{
  "type": "copy_text | navigate_page | open_channels_activity | open_mini_program | preview_image | open_webview",
  "require_auth": false,
  "payload": {
    "text": "网盘链接或口令",
    "toast": "链接已复制，请打开浏览器访问",
    "page_id": "drama_detail",
    "feed_id": "export/UzFoc...",
    "finder_user_name": "sph...",
    "target_app_id": "wx1234567890abcdef",
    "target_path": "pages/index/index?from=matrix_app_a",
    "extra_data": { "channel": "hot_keyword" },
    "env_version": "release"
  }
}
```

### 3.2 原子积木定义 (`BlockItem`)
每一个原子组件由 4 个要素构成：
```json
{
  "id": "block_hero_101",
  "type": "media_hero",
  "props": {
    "title": "猴王下山",
    "subtitle": "第 1 集试看 · 全网爆火",
    "cover_url": "https://.../cover.jpg",
    "video_url": "https://.../video.mp4"
  },
  "style": {
    "margin_y": "24rpx",
    "border_radius": "32rpx",
    "glass_blur": true,
    "accent_color": "#FF9F0A"
  },
  "action": {
    "type": "open_channels_activity",
    "payload": { "feed_id": "...", "finder_user_name": "..." }
  }
}
```

### 3.3 页面配置协议 (`DynamicPage`)
描述一个完整页面的全局元信息：
```json
{
   "schema_version": 3,
   "page_id": "home",
   "title": "猴王下山 - 精选剧场",
   "business_type": "drama",
   "intent": "watch",
   "theme": "dark_glass",
  "accent_color": "#FF9F0A",
  "share_config": {
    "friend": {
      "enabled": true,
      "title": "猴王下山全集免费看",
      "path": "/pages/dynamic/index?page_id=drama_detail&id=101",
      "image_url": "https://.../share.jpg"
    },
    "timeline": {
      "enabled": true,
      "title": "猴王下山全集免费看",
      "query": "page_id=drama_detail&id=101&from=timeline",
      "image_url": "https://.../share.jpg"
    }
  },
  "blocks": [
    /* 有序的 BlockItem 列表 */
  ]
}
```

### 3.4 统一响应信封与版本协商

页面接口不直接返回裸页面，统一使用响应信封，便于缓存、灰度、回滚和客户端兼容：

```json
{
  "protocol_version": "1.1",
  "schema_version": 3,
  "request_id": "req_01J...",
  "page": {
    "page_id": "drama_detail",
    "revision": 42,
    "status": "published",
    "title": "{{entity.title}}",
    "business_type": "drama",
    "intent": "watch",
    "require_auth": false,
    "blocks": []
  },
  "data": {},
  "capabilities_required": ["video", "clipboard"],
  "cache": { "etag": "page-42", "max_age": 30 },
  "fallback": { "page_id": "home", "mode": "static_safe" }
}
```

客户端请求时携带 `X-SDUI-Version` 和 `X-Client-Capabilities`。服务端只下发客户端声明支持的块和动作；不支持时使用块级 `fallback`，不能因为一个新块导致整页白屏。

### 3.5 数据绑定与数据源

块不能只依赖字符串替换。统一采用受控绑定路径，路径只读、可审计，不执行 JavaScript、SQL 或模板表达式：

- `$page.*`：页面元信息；
- `$query.*`：URL 查询参数；
- `$entity.*`：后端按业务类型装配的实体；
- `$item.*`：列表循环项；
- `$session.*`：当前登录态的非敏感字段；
- `$tenant.*`：当前小程序的公开配置。

数据源只允许后端注册的实体和查询方式，例如 `drama`、``、`score_result`、`download_resource`。协议只传 `entity`、`id`、`fields`、`filters`、`cursor`、`limit`，禁止传任意 SQL。敏感资源（网盘真实地址、兑换码、手机号等）必须由后端鉴权后单独返回，不能提前放在公开页面 JSON 中。product

### 3.6 块、容器、循环和条件

将“原子积木”分为三类，避免每新增一个业务就新增一套页面：

1. **布局块**：`stack`、`container`、`grid`、`tabs`、`carousel`、`list`、`spacer`；
2. **内容块**：`text`、`rich_text`、`image`、`media_hero`、`video`、`notice`、`timeline`、`empty`、`skeleton`；
3. **业务块**：`resource_card`、`episode_list`、`score_panel`、`coupon_card`、`countdown`、`result_table`、`form`、`contact_card`、`map_card`、`game_card`、`game_header`、`redeem_code_card`、`server_status`、`product_card`、`download_card`、`event_card`、`poll`、`feed_list`。

`gallery`、`filter_bar`、`anchor_nav` 等通用列表辅助块可作为 `container` 的受控子类型；块注册表必须明确每个子类型的最小客户端版本。

块统一支持 `visible_when`、`repeat`、`loading`、`empty`、`error` 和 `fallback`。条件表达式只提供有限操作符：`eq`、`neq`、`in`、`exists`、`gt`、`gte`、`lt`、`lte`、`and`、`or`、`not`。示例：

```json
{
  "id": "unlock_button",
  "type": "action_button",
  "visible_when": { "and": [
    { "eq": [{ "path": "$entity.is_locked" }, true] },
    { "exists": { "path": "$entity.resource_id" } }
  ]},
  "props": { "text": "获取后续" },
  "action": { "type": "request_data", "payload": { "resource_id": { "path": "$entity.resource_id" } } }
}
```

### 3.7 事件、动作编排与页面状态

`action` 不再限定为单个动作，块通过 `events` 绑定动作列表。动作列表按顺序执行，支持 `condition`、`confirm`、`on_success`、`on_error` 和埋点字段：

```json
{
  "events": {
    "tap": [
      { "type": "require_auth" },
      { "type": "request_data", "payload": { "resource_id": { "path": "$entity.resource_id" } } },
      { "type": "copy_text", "payload": { "path": "$result.copy_text" } },
      { "type": "toast", "payload": { "text": "已复制" } }
    ]
  }
}
```

页面状态只允许存在于当前页面命名空间，提供 `set`、`toggle`、`reset` 三种变更，禁止动作直接修改全局对象。跨页只通过 URL 参数或服务端结果传递，避免状态污染。列表统一支持 `load_more` 和 `refresh`，并规定 `cursor`、`has_more`、去重键和最大条数。

### 3.8 自定义请求动作

页面可以把点击事件绑定到后端业务接口，支持查询、领取、报名、预约、兑换码核验等场景。为了避免把小程序变成开放代理，协议配置的是**已登记的接口端点**，而不是任意互联网 URL：

```json
{
  "type": "request_data",
  "payload": {
    "endpoint": "game.redeem",
    "url": "/api/v1/actions/game/{game_id}/redeem",
    "method": "POST",
    "path_params": {
      "game_id": { "path": "$entity.game_id" }
    },
    "query": {
      "channel": "wechat_search"
    },
    "body": {
      "code": { "path": "$state.form.code" }
    },
    "response": {
      "data_path": "data",
      "save_as": "redeem_result"
    },
    "on_success": [
      { "type": "toast", "payload": { "text": "兑换成功" } },
      { "type": "refresh", "target": "redeem_status" }
    ],
    "on_error": [
      { "type": "show_error_state", "target": "redeem_form" }
    ]
  }
}
```

请求动作的标准字段：

- `endpoint`：推荐配置，使用后端登记的端点名称，由服务端维护真实地址、方法和凭证；
- `url`：需要自定义地址时使用。允许同源相对路径，或租户请求域名白名单中的 HTTPS 地址；禁止内网地址、动态协议、URL 中携带凭证以及绑定整个 URL；
- `method`：支持 `GET`、`POST`、`PUT`、`PATCH`、`DELETE`，默认 `GET`。修改和删除类请求必须先确认，并由后端执行权限和幂等校验；
- `path_params`、`query`、`body`：分别配置路径参数、查询参数和 JSON 请求体；值可以来自受控绑定路径、页面状态或固定白名单值，并在请求前通过参数 Schema 校验；
- `headers`：只允许 `Content-Type`、`Accept-Language`、`Idempotency-Key` 等安全白名单字段；租户、客户端版本和登录令牌由统一请求层自动注入；
- `response.data_path`、`save_as`：将响应中的非敏感数据保存到当前页面状态，供后续块和动作使用；
- `on_success`、`on_error`：请求完成后的动作链，支持提示、刷新、跳转、复制和展示状态；
- `require_auth`、`timeout_ms`、`idempotency_key`：分别控制鉴权、超时和重复提交；
- `loading`、`empty`、`error`：请求期间及异常时绑定到对应块的状态视图。

当 `endpoint` 与 `url` 同时存在时，以 `endpoint` 注册表解析结果为准，`url` 只作为管理后台预览信息，避免配置被篡改。端点注册表还必须声明请求方法、参数 Schema、响应 Schema、最低客户端版本、限流策略和是否允许匿名访问。客户端不得接收服务端密钥，不得配置任意 `Authorization`、Cookie 或内部请求头；上传文件、支付和敏感数据提交应使用专门的业务动作。所有真实域名仍须加入微信小程序 `request` 合法域名。

### 3.9 动作能力矩阵与安全边界

动作类型使用稳定的蛇形命名：`copy_text`、`navigate_page`、`open_channels_activity`、`open_mini_program`、`preview_image`、`open_webview`、`request_data`、`require_auth`、`toast`、`refresh`、`share`、`subscribe_message`。每个动作必须声明：

- 支持的平台和最低基础库；
- 必填参数及类型；
- 失败时的用户提示和降级动作；
- 域名、AppID、页面路径等白名单约束。

协议不得下发任意脚本、远程组件、任意小程序路径或任意网页域名。`open_webview` 只能使用已配置的业务域名，`open_mini_program` 只能跳转已审核的 AppID 和路径。动作参数的字段命名必须统一，不能同时出现 `feed_id` 与 `feedId`。

### 3.10 页面分享协议

分享配置属于 `DynamicPage`，每个页面都可以独立设置分享给朋友和分享到朋友圈的参数；未配置时使用租户默认值，禁止由客户端自行拼接业务敏感参数：

```json
{
  "share_config": {
    "default_image_url": "https://.../share-default.jpg",
    "friend": {
      "enabled": true,
      "title": "绝地突围礼包码领取",
      "path": "/pages/dynamic/index?page_id=game_redeem&game_id=101",
      "image_url": "https://.../game-share.jpg",
      "query": "from=friend"
    },
    "timeline": {
      "enabled": true,
      "title": "绝地突围礼包码领取",
      "query": "page_id=game_redeem&game_id=101&from=timeline",
      "image_url": "https://.../game-share.jpg"
    }
  }
}
```

- `friend` 对应 `onShareAppMessage`：支持 `title`、`path`、`image_url`；`query` 可并入 `path`，服务端应统一编码并校验长度；
- `timeline` 对应 `onShareTimeline`：微信只接受 `title`、`query`、`image_url`，没有独立的 `path` 字段；
- `enabled` 为 `false` 时，页面不主动声明该分享入口；系统仍须遵循微信客户端实际展示规则；
- 分享参数支持 `$entity`、`$query`、`$tenant` 的受控绑定，但不得放入 token、手机号、兑换码等敏感信息；
- 每次分享自动附带不可伪造的 `campaign_id`、`share_id` 和来源标记，用于归因、去重和回访；
- 页面跳转后的分享配置必须重新从目标页面读取，不能沿用上一个页面的状态。

### 3.11 登录态生命周期与内置刷新机制

客户端不能用“本地是否存在 token”判断登录有效性。登录态至少分为短期 `access_token` 和长期 `refresh_token`：

```json
{
  "access_token": "jwt...",
  "access_expires_at": "2026-09-03T20:00:00+08:00",
  "refresh_token": "opaque...",
  "refresh_expires_at": "2026-10-03T19:00:00+08:00",
  "session_id": "sess_01J..."
}
```

- `access_token` 用于普通 API，过期时间较短；服务端每次请求校验签名、过期时间、`app_id`、用户和会话状态；
- `refresh_token` 只用于刷新，不作为业务接口凭证。服务端保存其哈希、租户、用户、设备/会话和过期时间，不保存明文；
- 每次刷新都轮换 `refresh_token`，旧 refresh token 立即失效；发现旧 token 重复使用时，撤销该会话下的全部 token；
- access/refresh token 必须绑定 `app_id` 和 `session_id`，不能跨小程序或跨会话使用；
- 登出、修改密码、账号封禁、管理员强制下线和检测到 refresh token 重放时，服务端撤销会话。

推荐接口：

- `POST /api/v1/auth/wechat-login`：使用微信 `code` 创建会话并返回双 token；兼容旧版 `/api/v1/code` 时不得继续签发只有单一 JWT 的长期登录态；
- `POST /api/v1/auth/refresh`：提交 refresh token，返回新的 access/refresh token；
- `GET /api/v1/auth/session`：返回当前会话有效性、用户和过期时间，用于客户端启动恢复；
- `POST /api/v1/auth/logout`：撤销当前 session；

客户端内置流程：

```text
启动应用
  -> 读取本地会话
  -> 无会话/refresh 已过期：微信登录
  -> access 未过期：直接请求
  -> access 已过期或服务端返回 401：refresh
  -> refresh 成功：原请求只重试一次
  -> refresh 失败：清除本地会话并重新微信登录
```

实现要求：

- 本地解析 JWT 的 `exp` 只能作为提前刷新优化，不能代替服务端校验；考虑客户端时钟偏差，提前 `60~120` 秒刷新；
- 应用启动可以调用 `auth/session` 做一次权威校验，但不能因为每个页面进入都重复登录；
- 同一时刻多个请求遇到 401 时只允许一个 refresh 请求，其余请求等待同一个结果，避免刷新风暴；
- refresh 失败后的原请求不得无限重试，最多一次；网络错误与身份失效要区分处理；
- access token、refresh token 只存入小程序安全存储，不写入 URL、分享参数、日志、截图 Fixture 或页面协议；
- `require_auth` 页面和动作在 refresh 失败后回到统一登录流程，登录成功后恢复原页面或原动作一次，恢复失败则显示明确错误态；
- 页面仅消费 `$session.is_authenticated`、`$session.user_id` 等非敏感状态，不读取 token 原文。

### 3.12 AI/MCP 编排接口

SDUI 从实现之初就提供 MCP 接口，让 AI 能够通过结构化工具快速搭建页面、调用真实数据预览并进行视觉审查。MCP 只操作受控的页面草稿和模板，不直接修改数据库，不绕过发布审批。

建议提供以下 MCP 工具：

| 工具 | 作用 | 关键输入 | 关键输出 |
|---|---|---|---|
| `sdui.template.list` | 查询可用行业模板和版本 | `business_type`、`keyword` | 模板摘要、适用场景、所需实体 |
| `sdui.page.create` | 从模板或空白页面创建草稿 | `template_id`、`page_id`、`context` | `draft_id`、初始 `DynamicPage` |
| `sdui.page.patch` | 按路径修改页面协议 | `draft_id`、JSON Patch、操作者 | 新 revision、校验结果 |
| `sdui.page.validate` | 校验协议、绑定、动作、权限和能力 | `draft_id` 或协议 JSON | 错误、警告、自动修复建议 |
| `sdui.page.preview` | 使用指定数据和客户端能力渲染预览 | `draft_id`、`device`、`fixtures` | 预览地址、渲染日志、最终协议 |
| `sdui.page.screenshot` | 后端生成规范化截图 | `draft_id`、设备尺寸、状态 | PNG 地址、截图哈希、视觉报告 |
| `sdui.page.publish` | 发布已通过校验的版本 | `draft_id`、灰度范围、备注 | `release_id`、生效时间、回滚版本 |

MCP 工具必须具备以下约束：

- 所有写操作默认只创建或修改草稿；发布、撤回和回滚必须显式调用并记录审计信息；
- `sdui.page.patch` 使用 JSON Patch 或受控路径操作，不能让 AI 提交任意 SQL、脚本、组件代码或数据库字段；
- 工具返回机器可读的错误码、字段路径和修复建议，便于 AI 自动迭代，而不是只返回自然语言错误；
- AI 可以读取模板、协议、校验结果和截图，但默认不能读取 AppSecret、用户隐私和未授权的敏感资源；
- 每次 AI 修改生成 `revision`，支持差异查看、撤销和回滚；并发修改使用乐观锁，避免覆盖人工编辑。

MCP 服务本身需要独立的身份认证和权限范围：`read`（模板、协议、截图）、`write:draft`（创建和修改草稿）、`release`（发布/撤回/回滚）。生产环境只允许通过受保护的 MCP 传输端点接入，按租户和操作者隔离；每次工具调用记录 `request_id`、`actor_id`、`tenant_id`、输入摘要和结果 revision。MCP 不应直接暴露数据库连接或内部管理 API，工具层负责参数校验、脱敏和审计。

AI 快速搭建的标准流程：

```text
选择模板/业务类型 -> 生成草稿 -> 绑定实体与请求 -> 协议校验
-> 后端预览 -> 后端截图 -> AI 视觉审查 -> 修正草稿
-> 人工确认 -> 发布/灰度 -> 采集效果
```

### 3.13 后端截图与视觉一致性协议

截图不是独立的设计稿，也不是管理后台自行拼接的图片，而是后端对同一份已解析 `DynamicPage` 执行规范化渲染得到的视觉基线。协议解析、绑定、条件求值、状态合并必须先生成统一的中间表示（IR）；小程序渲染器和截图渲染器都消费这份 IR，不能各自重新解释 JSON。截图服务必须记录：`protocol_version`、`schema_version`、`revision`、`device`、`theme`、`locale`、`data_fixture_id` 和渲染器版本。

视觉一致性分为三层：

1. **协议一致**：截图使用的页面 JSON、块顺序、绑定后的值、条件结果和动作状态，与实际客户端收到的协议完全相同；
2. **布局一致**：前端和后端共享同一套 Design Tokens、块尺寸规则、字体回退、间距、圆角、颜色和状态占位尺寸；
3. **状态一致**：加载中、空结果、错误、登录拦截、库存不足、兑换成功、视频不可用等状态都能单独截图，不能只审查成功态。

后端渲染器与小程序原生能力存在差异时，必须使用确定性的能力替身，例如将视频号播放器渲染为同尺寸的封面和状态占位，但不得改变块的尺寸、边距、按钮位置或文字层级。截图报告应同时列出“原生能力替身”清单，避免 AI 将占位内容误判为协议缺陷。替身只用于截图和预览，真实小程序运行时仍由对应原生组件执行。

截图接口至少支持：设备宽度/高度、DPR、横竖屏、主题、语言、登录态、查询参数、实体 Fixture、网络状态和指定页面状态。截图结果除了 PNG，还应返回可供 AI 分析的结构树：块 ID、类型、边界框、可见性、文本摘要、动作类型和异常列表。

视觉验收最低标准：

- 同一 revision 在相同设备和 Fixture 下截图哈希稳定；
- 未知块、缺失字段或能力不足时有明确降级，不得出现空白页或布局塌陷；
- 文本不溢出、不遮挡、不被固定底栏覆盖，长标题和多语言均需验证；
- 关键点击目标有稳定尺寸，截图中的块 ID 可以映射回协议路径；
- 发布前必须通过协议校验和截图审查，视觉报告与协议 revision 不一致时禁止发布。

---

## 四、核心技术难点解决方案

### 4.1 样式怎么做？—— 苹果 Design Tokens 视觉标记系统
- **原则**：非开发人员不能随心所欲写 CSS，以免破坏 UI 规范或引发跨端错位。
- **机制**：前端预装一套工业级 SCSS 苹果规范样式库。后台仅开放可视化 Tokens 调节：
  - **主题基调**：`dark_glass` (深黑磨砂)、`light_clean` (极简冷白)、`cyber_neon` (赛博霓虹)；
  - **材质质感**：高斯模糊毛玻璃 (`backdrop-filter: blur(20px)`)、纯色磨砂、高光描边；
  - **圆角梯度**：直角(0)、轻微(16rpx)、标准(28rpx)、胶囊全圆(999rpx)；
  - **渐变微调**：苹果琥珀橙红渐变、科技青蓝渐变、高光白炽。
- **效果**：无论后台如何排列组合，渲染出来永远是纯正高雅的苹果原生质感。

---

### 4.2 多页面跳转与参数流转怎么做？—— 万能动态页面容器
小程序无法在运行时动态添加新路由页面，为此在前端固定注册两个万能容器：
1. **主页容器**：`pages/index/index`（默认拉取 `page_id=home`）；
2. **子页容器**：`pages/dynamic/index`（读取 URL 中的 `?page_id=xxx` 动态渲染）。

#### 跳转执行流：
1. 用户在 Page A 点击按钮，其 `action` 为 `{ type: "navigate_page", payload: { page_id: "download_detail", id: 101 } }`；
2. 动作分发器调用原生路由：
   ```typescript
   Taro.navigateTo({ url: '/pages/dynamic/index?page_id=download_detail&id=101' })
   ```
3. 微信小程序原生压栈新页面，自带顶部返回键 `<`；
4. 目标页面加载，将 `id=101` 传给接口拉取专属组装数据，无缝渲染。

---

### 4.3 动态详情页怎么做？—— 模版 + 变量插值引擎 (Data Binding)
- **痛点**：100 部短剧或 1000 个商品，不需要建 1000 个页面，只需建 **1 套通用详情模版**。
- **后端数据装配**（字符串插值仅作为旧协议兼容层，不作为新模板实现）：
  当请求 `GET /api/v1/page/drama_detail?id=101` 时：
  1. 后端取出 `drama_detail` 的基础积木；
  2. 根据 `id=101` 查出该短剧的真实数据（剧名、封面、选集、专属资源标识、视频号 `channels_feed_id`）；
  3. 后端按照 3.5 节的受控绑定路径装配数据：
     - `$entity.title` 绑定真实剧名；
     - `$entity.cover_url` 绑定真实封面；
     - `$entity.resource_id` 绑定资源标识，资源内容在鉴权动作中获取；
  4. 将装配好的完整 Blocks 返回给小程序前端；
  5. 前端只管无脑原生渲染，逻辑极其轻量干净！

---

## 五、多小程序矩阵“一脑多控”（Multi-Tenant 多租户隔离）

针对用户“**一个后端管理几十个小程序、每个小程序独立改名设置、批量承接不同上升指数词**”的核心商业诉求，系统采用标准的多租户（AppID 隔离）架构：

### 5.1 核心隔离设计
1. **多应用配置表 (`mini_apps`)**：
   ```go
   type MiniApp struct {
       AppID        string    `gorm:"primaryKey;size:64;comment:小程序AppID"`
       AppSecret    string    `gorm:"size:64;comment:小程序密钥(换取openid)"`
       AppName      string    `gorm:"size:128;comment:小程序名称(当前蹭的热词)"`
       CurrentPage  string    `gorm:"size:64;default:'home';comment:当前线上激活的主页ID"`
       ReleaseMode  string    `gorm:"size:16;default:'normal';comment:发布模式(normal/gray/fallback)"`
       FallbackPageID string  `gorm:"size:64;default:'home';comment:故障或过期时的兜底页面"`
       CreatedAt    time.Time
       UpdatedAt    time.Time
   }
   ```
2. **页面协议按 `app_id` 物理隔离 (`dynamic_pages`)**：
   - 联合主键 / 联合唯一索引：`(app_id, page_id)`；
   - 小程序 A 和小程序 B 各自拥有独立的 `home` 主页、独立的模版配置、互不干扰。
3. **接口请求头自适应识别**：
   - 小程序前端发起的任何请求，由 `request.ts` 统一在 Header 自动附带：
     `X-App-Id: wx516563cfe994bbc6`
   - 后端中间件提取 `X-App-Id`，精准装配并返回属于该小程序的专属页面配置；
4. **管理后台一脑多控切换器**：
   - `/admin` 顶部提供**小程序选择下拉框**：
     `[ 📱 小程序 1: 猴王下山 (wx516...) ▼ ]`
   - 切换后，左侧 iPhone 模拟器和右侧积木编辑器立即切换为该小程序的数据，实现“单点后台统一操盘几十个小程序”。

---

## 六、页面级登录态与微信鉴权闭环 (Auth & JWT Lifecycle)

某些高价值场景（如复制核心解密资源、领取大额兑换码、查看个人中心、提交查询表单）需要强制用户登录或绑定身份，系统建立完整的**页面级与动作级鉴权闭环**：

### 6.1 鉴权配置声明 (Schema 扩展)
- **页面级受保声明**：`DynamicPage` 增加 `require_auth: true / false`；
- **动作级受保声明**：`BlockAction` 增加 `require_auth: true / false`（未登录时点击该按钮先拦截并弹窗引导登录）。

### 6.2 微信原生免密登录时序闭环
```
  微信小程序前端                        Golang 后端服务                  微信官方服务器
      │                                       │                                │
      │ 1. 进入受保页面 / 点击受保按钮       │                                │
      │ 2. 检查本地 access/refresh 会话       │                                │
      │    access 过期则先调用 refresh         │                                │
      │    refresh 失败才唤起 Taro.login()      │                                │
      │                                       │                                │
      │ 3. POST /api/v1/auth/wechat-login ───>│                                │
      │    携带 { app_id, code }              │                                │
      │                                       │ 4. 查出 app_id 对应的 Secret   │
      │                                       │    请求 jscode2session ───────>│
      │                                       │<── 返回 { openid, session_key }│
      │                                       │                                │
      │                                       │ 5. 写入 users 表 (app_id+openid)│
      │                                       │ 6. 创建 session 并签发双 token │
      │<── 返回 { access, refresh, user } ───│                                │
      │                                       │                                │
      │ 7. 本地持久化安全会话                 │                                │
      │ 8. 重发受保页面/动作请求             │                                │
      │    Header: Authorization: Bearer ... ─> 9. 服务端校验 session 并放行  │
```

### 6.3 跨小程序多租户用户数据隔离
- 数据库 `users` 表结构升级为以 `(app_id, wechat_openid)` 联合唯一；
- JWT Token 载荷中内嵌 `app_id` 与 `open_id`；
- 避免不同小程序之间的用户数据或积分冲突。

---

## 七、双端同构与极客级复用（Taro React + Web 管理后台）

### 7.1 共享协议与组件能力
在 `minifront/src/components/SDUI/` 内部开发原子组件：
- **`MediaHeroBlock`**：大视频播放 / 视频号内嵌 / 大海报；
- **`ResourceCardBlock`**：网盘多渠道提取卡片（夸克/百度/迅雷）；
- **`ActionButtonBlock`**：通栏大胶囊主按钮；
- **`ItemGridBlock`**：自适应 2/3/4 列网格（集数、画廊、九宫格、壁纸）；
- **`CopyListBlock`**：兑换码 / 口令复制列表；
- **`TimelineBlock`**：吃瓜始末 / 热点时间线；
- **`AnnouncementBar`**：顶部跑马灯通告。
- **`FormBlock`**：查询、报名、预约和反馈表单。

### 7.2 动作分发器的环境自适应
```typescript
export const dispatchAction = async (action?: BlockAction) => {
  if (!action) return
  const isWeapp = Taro.getEnv() === Taro.ENV_TYPE.WEAPP

  // 1. 拦截动作级登录鉴权
  if (action.require_auth) {
    const sessionReady = await ensureSession()
    if (!sessionReady) return
  }

  // 2. 执行具体业务行为
  switch (action.type) {
    case 'copy_text':
      Taro.setClipboardData({
        data: action.payload.text,
        success: () => Taro.showToast({ title: action.payload.toast || '已复制', icon: 'success' })
      })
      break
    case 'open_channels_activity':
      if (isWeapp) {
        wx.openChannelsActivity({
          feedId: action.payload.feed_id,
          finderUserName: action.payload.finder_user_name
        })
      } else {
        Taro.showToast({ title: `[模拟] 打开视频号: ${action.payload.feed_id}`, icon: 'none' })
      }
      break
    case 'navigate_page':
      Taro.navigateTo({
        url: `/pages/dynamic/index?page_id=${action.payload.page_id}&id=${action.payload.id || ''}`
      })
      break
    case 'open_mini_program':
      // 跨小程序矩阵联动跳转 (流量互导与分流)
      if (isWeapp) {
        Taro.navigateToMiniProgram({
          appId: action.payload.target_app_id,
          path: action.payload.target_path || '',
          extraData: action.payload.extra_data || {},
          envVersion: action.payload.env_version || 'release',
          fail: (err) => console.warn('跳转小程序失败:', err)
        })
      } else {
        Taro.showToast({
          title: `[模拟跳转小程序] AppID: ${action.payload.target_app_id}`,
          icon: 'none'
        })
      }
      break
  }
}
```

---

## 八、指数词业务分支与通用漏斗

指数词的共同打法不是“换一张海报”，而是把搜索意图映射到一条可配置漏斗：

```text
指数词/入口参数 -> 意图识别 -> 首屏结果 -> 详情或操作 -> 转化/留资 -> 分享与回访
```

`DynamicPage` 增加以下上下文，页面模板不绑定具体行业：

```json
{
  "business_type": "drama",
  "intent": "watch",
  "keyword": "猴王下山",
  "source": "wechat_search",
  "campaign_id": "hot_20260903",
  "expires_at": "2026-09-06T23:59:59+08:00"
}
```

可复用的业务分支如下：

| 分支 | 典型指数词 | 首屏重点 | 主要动作 | 需要的业务块 |
|---|---|---|---|---|
| 内容/短剧 | 剧名、电影、动漫、综艺 | 播放入口、更新状态、目录 | 播放、选集、分享、获取全集 | `media_hero`、`episode_list`、`resource_card` |
| 游戏/礼包 | 游戏名、新游、兑换码、攻略、开服 | 游戏入口、礼包状态、版本、活动 | 查看详情、领取、复制、进入游戏 | `game_card`、`game_header`、`redeem_code_card`、`server_status` |
| 查询/结果 | 成绩查询、天气、物流、榜单 | 结果状态、更新时间、关键字段 | 查询、刷新、订阅提醒 | `form`、`result_table`、`timeline` |
| 下载/资源 | 软件安装包、电子书、壁纸、游戏兑换码 | 版本、适用平台、资源大小 | 下载、复制口令、领取 | `resource_card`、`copy_list`、`download_card` |
| 交易/优惠 | 商品名、优惠券、活动、价格 | 价格、库存、优惠截止时间 | 领券、跳转购买、分享 | `product_card`、`coupon_card`、`countdown` |
| 服务/预约 | 医院、门店、客服电话、办事入口 | 地址、营业状态、服务时间 | 拨号、地图、预约、客服 | `contact_card`、`map_card`、`form` |
| 活动/报名 | 演唱会、展会、考试报名、直播 | 时间、地点、名额、报名状态 | 报名、日历提醒、直播 | `event_card`、`form`、`countdown` |
| 社区/互动 | 投票、问答、攻略、热点事件 | 观点摘要、热度、参与入口 | 投票、评论、收藏、举报 | `poll`、`feed_list`、`notice` |

每个分支只新增后端实体装配器和少量业务块，不新增一套路由。统一保留以下通用能力：关键词及来源追踪、有效期、内容审核状态、分享参数、登录要求、转化事件埋点和失效后的兜底页。

### 8.1 游戏业务页面族

游戏不是单独的一套渲染引擎，而是通用页面容器、数据绑定和动作系统上的一个 `business_type=game` 分支。建议使用以下页面模板：

| 页面 | `page_id` 示例 | 核心数据 | 主要块和动作 |
|---|---|---|---|
| 游戏主页 | `game_home` | 推荐游戏、热门礼包、开服列表、活动 Banner | `carousel`、`game_card`、`redeem_code_card`、`server_status` |
| 游戏列表 | `game_list` | 分类、平台、标签、分页游戏列表 | `tabs`、`filter_bar`、`list`、`game_card` |
| 游戏详情 | `game_detail` | 图标、截图、简介、版本、厂商、评分、下载/跳转信息 | `game_header`、`gallery`、`rich_text`、`action_button` |
| 兑换码领取 | `game_redeem` | 礼包库存、领取条件、有效期、用户领取状态 | `redeem_code_card`、`form`、`countdown`、`request_data` |
| 攻略/资讯 | `game_guide` | 攻略正文、目录、关联游戏、更新时间 | `rich_text`、`anchor_nav`、`game_card`、`share` |
| 开服/活动 | `game_event` | 开始时间、服务器、预约状态、活动规则 | `timeline`、`server_status`、`countdown`、`subscribe_message` |

页面跳转仍使用万能容器：

```json
{
  "type": "navigate_page",
  "payload": {
    "page_id": "game_detail",
    "query": { "game_id": { "path": "$item.id" } }
  }
}
```

游戏实体建议至少包含 `game_id`、`name`、`icon_url`、`cover_url`、`screenshots`、`platforms`、`category`、`tags`、`version`、`publisher`、`description`、`status` 和允许的入口动作。兑换码实体建议包含 `package_id`、`game_id`、`title`、`remaining`、`claim_status`、`expires_at`、`claim_requirements`，但未领取前不返回真实兑换码。

兑换码领取必须走受保护的 `request_data` 端点，由服务端在同一事务中完成资格校验、库存扣减、领取记录和兑换码分配。请求必须携带幂等键，重复点击返回同一次领取结果；领取成功后才把兑换码返回当前页面状态，再由 `copy_text` 复制。客户端显示的库存只用于展示，不能作为扣减依据。

### 8.2 分支配置规则

- `business_type` 决定允许的数据源和业务块集合；
- `intent` 决定首屏排序和主动作，例如 `watch`、`query`、`download`、`redeem`、`buy`、`book`、`join`；
- `campaign_id` 用于灰度、渠道归因和一键回滚；
- `expires_at` 到期后自动切换到安全兜底页，不能继续展示过期资源；
- 不同分支共用 `stack/list/form/action` 等基础块，只有领域数据和少数业务块不同。

### 8.3 协议必须覆盖的非正常分支

协议除了“成功展示”还必须定义：无结果、数据过期、部分字段缺失、登录失败、权限不足、接口超时、客户端能力不足、内容被下架和网络离线。每种情况都要有 `empty/error/fallback` 块或页面级兜底，且不能把后端错误文本直接当作用户文案。

## 九、实施计划

实施遵循“先协议与安全边界，再渲染闭环，最后模板和 AI 自动化”的顺序。每个阶段必须有可运行的增量产物，不能先做后台编辑器再倒推协议。

### 9.1 阶段与交付物

| 阶段 | 目标 | 主要工作 | 必须交付 |
|---|---|---|---|
| 0. 基线冻结 | 明确边界和兼容策略 | 冻结协议 v1、块/动作/绑定枚举；确认旧 `/drama/home` 兼容期；定义错误码、能力矩阵和安全白名单 | JSON Schema、协议示例、兼容策略、评审记录 |
| 1. 协议内核 | 后端能安全解析和装配页面 | 实现协议校验、绑定、条件求值、状态模型、动作 Schema、版本协商和 IR；未知字段可忽略、未知块可降级 | Go 协议包、契约测试、IR 示例、错误码表 |
| 2. 数据与发布 | 页面可持久化、发布和回滚 | 建立租户、模板、页面草稿、页面版本、端点注册、发布记录、会话模型；实现缓存、灰度、撤回、回滚和审计 | 数据库迁移、页面 API、发布 API、回滚演示 |
| 3. 小程序运行时 | 客户端能渲染通用页面 | 注册动态页面容器；实现块注册表、绑定/条件渲染、加载/空/错态、动作分发、分享、能力降级和登录刷新 | `pages/dynamic/index`、运行时组件、真机包 |
| 4. 后端截图 | 生成可信视觉基线 | 使用同一 IR 实现规范化截图；支持设备、主题、语言、Fixture、登录态和异常状态；输出 PNG、结构树和哈希 | 截图 API、视觉报告、固定 Fixture 集 |
| 5. 模板包 | 快速搭建常见行业页面 | 实现 `drama`、`game`、`query`、`download` 模板；覆盖游戏主页、游戏详情、兑换码领取等页面族；支持模板生成后逐块自定义 | 模板注册表、模板版本、示例页面、迁移规则 |
| 6. MCP/AI | AI 可搭建和审查页面 | 实现模板查询、创建草稿、Patch、校验、预览、截图和发布工具；接入权限、审计和人工确认 | MCP 工具清单、机器错误码、AI 搭建样例 |
| 7. 运营与上线 | 支持热词快速响应 | 完成多租户后台、灰度发布、指标埋点、过期兜底、内容审核和告警；建立上线手册和回滚演练 | 运行手册、监控面板、应急预案、上线检查表 |

### 9.2 依赖与门禁

- 阶段 0 未通过，禁止开始模板和后台开发；
- 阶段 1 未通过，禁止让前端直接消费生产页面 JSON；
- 阶段 2 未通过，禁止开放生产发布或让 AI 直接改线上版本；
- 阶段 3、4 的协议 IR、块边界和状态必须一致，未通过视觉对照不得进入模板批量生成；
- 阶段 6 先开放 `read` 和 `write:draft`，`release` 权限必须最后开放并要求人工确认；
- 每阶段结束都保留可回滚版本、测试数据和变更记录。

### 9.3 最小首发范围

首个可上线版本只要求：

- 单租户到多租户的数据模型已预留，先启用一个小程序；
- `DynamicPage`、`BlockItem`、`BlockAction`、绑定、条件、请求和分享协议稳定；
- `drama`、`game` 两个模板包可用；
- 游戏主页、游戏详情、兑换码领取和短剧详情四个页面可从模板生成并逐块修改；
- access/refresh 登录态、后端截图、MCP 草稿编排和人工确认发布链路打通；
- `/api/v1/drama/home` 保留兼容，不阻塞现有客户端迁移。

## 十、验收标准

验收以“协议能否稳定表达、客户端能否一致呈现、线上能否安全运营”为准。以下标准全部满足才可进入生产灰度。

### 10.1 协议与兼容性

- 协议有可机读 JSON Schema；非法类型、缺失必填字段、未注册块和未注册动作均返回字段路径明确的错误或降级结果；
- 未知字段可忽略，未知块有 `fallback`，不会导致整页白屏；
- 旧客户端可读取兼容版本；新字段不会破坏旧字段语义；协议版本不兼容时返回明确的升级或兜底信息；
- 同一页面 revision 在相同租户、设备、主题、语言和 Fixture 下生成稳定 IR；
- 页面、模板、端点、动作和分享参数均有租户隔离，跨租户读取和修改测试必须失败。

### 10.2 页面与行业模板

- 不修改客户端代码，仅通过协议即可生成并切换短剧详情、游戏主页、游戏详情、兑换码领取、查询结果和下载资源页面；
- 模板生成页面与手工逐块搭建页面使用相同协议和渲染器；模板升级不覆盖已发布页面；
- `business_type` 只限制允许的数据源和业务块，不改变内核动作、状态、分享、埋点和降级逻辑；
- 页面支持长标题、缺图、空结果、数据过期、库存不足、下架和网络离线状态，布局不塌陷。

### 10.3 自定义请求与兑换码

- 请求动作支持登记端点及白名单 HTTPS/同源相对地址；支持方法、路径参数、query、body、参数绑定、响应映射和成功/失败动作链；
- 任意内网地址、任意脚本、任意请求头、任意凭证和未登记域名均被拒绝；
- 请求参数和响应按 Schema 校验，超时、限流、非 2xx、业务错误均进入协议定义的错误态；
- 兑换码领取具备资格校验、库存扣减、领取记录和幂等键；并发领取不能超发，重复请求返回同一领取结果；真实兑换码只在成功且授权后返回。

### 10.4 登录态与安全

- access token 过期但本地仍存在时，客户端能自动刷新，不要求用户重复操作；
- refresh token 过期、撤销或重放时，客户端清理会话并重新微信登录；
- 多个请求同时收到 401 时只产生一个 refresh 请求，原请求最多重试一次；
- Token 不出现在 URL、分享参数、日志、截图 Fixture、协议 JSON 或错误信息中；
- `app_id`、`session_id`、用户和权限在每次服务端鉴权中校验；登出和管理员强制下线立即生效；
- 管理后台和 MCP 写操作有角色权限、审计记录、乐观锁和发布确认。

### 10.5 分享与转化

- 每个页面可独立配置分享给朋友和朋友圈；朋友分享使用 `path`，朋友圈分享使用 `query`，字段符合微信客户端能力；
- 分享参数支持受控实体绑定和来源归因，不能包含敏感信息；目标页面打开后重新读取目标页面分享配置；
- 复制、领取、跳转、报名等关键动作有统一事件 ID、租户、campaign、页面 revision 和结果状态；
- 页面过期或活动下架后，旧分享链接进入安全兜底，不展示失效资源。

### 10.6 MCP、截图与视觉一致性

- AI 可以通过 MCP 完成“模板选择 -> 创建草稿 -> 修改 -> 校验 -> 预览 -> 截图 -> 再修改”的闭环；
- MCP 默认只能写草稿，发布需要明确权限和人工确认；工具调用有租户、操作者、请求 ID 和 revision 审计；
- 截图和小程序都消费同一份 IR；相同 Fixture 下，块顺序、可见性、文本、边界框、按钮位置和状态一致；
- 截图结果包含 PNG、哈希、结构树、渲染器版本、设备、主题、语言、Fixture 和原生能力替身清单；
- AI 视觉审查至少覆盖正常态、加载态、空态、错误态、登录拦截、过期态、库存不足和网络离线；
- 发现文字溢出、遮挡、点击区域不稳定、底栏覆盖或协议 revision 不一致时，发布自动阻断。

### 10.7 性能与可靠性

- 页面协议和已发布 IR 支持 ETag/版本缓存，发布后缓存可主动失效；
- 正常网络下首屏协议请求、解析和首屏渲染耗时有监控，P95 目标由部署规模确定并写入环境配置；
- 单个块接口失败不影响其他块呈现；页面级失败有兜底页；
- 发布、撤回和回滚为原子操作，回滚后新请求只能读取目标版本；
- Go 项目必须通过 `go build ./...`；服务层相关修改必须通过对应测试；小程序必须完成构建和至少一轮真机/模拟器测试。

### 10.8 验收证据

每项验收必须保留以下证据之一：自动化测试结果、接口请求/响应样例、协议 diff、截图及哈希、真机录屏、MCP 调用记录、审计日志或回滚演练记录。没有证据的“已支持”不视为验收通过。

## 十一、补充实施路线图（兼容旧版）

1. **第一阶段：多租户与鉴权底层 (Golang)**
   - 建立 `MiniApp` 模型与应用表迁移；
   - 建立 `DynamicPage` 模型支持 `app_id` 与 `require_auth`；
   - 升级微信登录接口支持多小程序 Secret 切换、session、access token 和 refresh token；
   - 实现 `refresh`、`session`、`logout`、会话撤销、refresh token 轮换和重放检测。

2. **第二阶段：Taro 前端通用动作与登录闭环**
   - 请求封装自动携带 `X-App-Id` 与 `Authorization: Bearer`；
   - 封装 access 过期预刷新、401 单次重试、并发刷新合并和微信登录回退；
   - 封装登录拦截浮层 / 静默登录重发机制；
   - 建立原子积木库与 `pages/dynamic/index` 容器。

3. **第三阶段：管理后台一脑多控工作台**
   - 增加多小程序快速切换下拉框；
   - 支持独立页面的积木拼装、样式微调、登录要求勾选与点击动作配置；
   - 支持受控发布、灰度、撤回和回滚；所有版本仍须遵守平台审核与内容规范。

4. **第四阶段：指数词业务模板**
   - 先实现 `drama`、`game`、`query`、`download` 四个分支；
   - 为每个分支注册实体装配器、允许块集合和主动作；
   - 补齐空态、过期、下架、能力不足和离线兜底；
   - 用同一套埋点字段比较不同模板的点击、完成和回访效果。

5. **协议验收**
   - 做 JSON Schema/契约测试、旧客户端兼容测试、租户隔离测试和真机能力矩阵测试；
   - 做发布、回滚、缓存失效、动作失败和敏感数据不下发测试；
   - 以“未知块不白屏、未知字段可忽略、旧协议可读取、过期页面可回退”为最低验收标准。

6. **MCP 与视觉回归**
   - 先实现只读的模板查询、协议校验、预览和截图工具，再开放草稿写入，最后接入发布权限；
   - 建立固定设备、主题、语言和 Fixture 的截图基线，按 `revision` 保存截图哈希和结构树；
   - 对协议 IR、截图和小程序真机结果做契约对照，发现块边界、文字层级、状态或动作映射不一致时阻止发布；
   - 为游戏主页、游戏详情、兑换码领取和至少一个自定义页面建立 AI 可重复执行的搭建与视觉审查样例。
