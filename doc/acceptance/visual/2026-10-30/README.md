<!-- README.md -->
# SDUI 真机视觉证据（2026-10-30）

环境：微信开发者工具 + `wechatide-skill` v0.3.9，AppID `wx516563cfe994bbc6`，页面 `component_lab_acceptance_20260907`，运行本地后端（`go run main.go`）。

本目录按设备/主题拆成四组证据，每组 34 张原始 PNG：

| 目录 | 设备 | 原始尺寸 | 说明 |
| --- | --- | --- | --- |
| `iphone-5-320x568/` | iPhone 5（320×568，Dpr 2） | 320×568 | 最窄屏回归，验证网格折叠、弹层底部定位与表单单列 |
| `iphone-12-390x844/` | iPhone 12/13 Pro（390×844，Dpr 3） | 363×785 | 深色主题常规屏回归 |
| `iphone-12-light-390x844/` | iPhone 12/13 Pro，浅色页 `component_lab_light_20261030` | 363×785 | 浅色主题全量回归 |
| `iphone-12-largefont-390x844/` | iPhone 12/13 Pro，微信字体大小 21 | 363×785 | 大字号回归（按钮、表单、弹层、列表） |

生成命令（设备切换后执行）：

```bash
pnpm --dir minifront run build:weapp
VISUAL_DEVICE="iPhone 5 320x568" \
VISUAL_OUTPUT_DIR="$PWD/doc/acceptance/visual/2026-10-30/iphone-5-320x568" \
node minifront/scripts/capture-sdui-visual.mjs

WECHAT_AGENT_PAGE_ID=component_lab_light_20261030 VISUAL_DEVICE="iPhone 12/13 Pro 390x844 light" VISUAL_OUTPUT_DIR="$PWD/doc/acceptance/visual/2026-10-30/iphone-12-light-390x844" node minifront/scripts/capture-sdui-visual.mjs
```

> 开发者工具会缓存 appservice 编译产物：改前端 JS 后仅 `simulator_refresh` 可能仍跑旧代码（WXSS 却会更新）。应先 `debug_clear_cache` 或删除 `WeappCache/WeappCompileCache/<项目哈希>`，再重开项目窗口。四组证据均在清缓存并确认新代码后生成。
>
> 截图尺寸由模拟器缩放决定：`WeappLocalData/localstorage_*.json` 中 `deviceScale:0.93`、`deviceScaleAutoFit:false`，并把 IDE 窗口调整到内容区至少 363×785（无需最大化或置顶），才能得到 iPhone 12/13 Pro 的 363×785；自动适配会缩成 309×667 或更小。

## 本组证据发现并修复的问题

1. **底部弹层在 320 宽屏被裁切**：dialog/action_sheet/bottom_sheet/filter_sheet 的 `position: fixed` 祖先链经过带 `backdrop-filter` 的毛玻璃容器，固定定位被限制在容器内，面板落到页面坐标底部。现统一用 `RootPortal` 把弹层挂到页面根节点，各组视图下面板均完整显示；弹层打开时的底层滚动锁仍验证通过（`scroll-y=false → true`）。
2. **网格 `1fr` 被内容撑破**：`grid-template-columns: repeat(n, 1fr)` 的最小尺寸是 min-content，表单/购物车等内容会把列撑宽并横向溢出。改为 `minmax(0, 1fr)`。
3. **窄屏密集块挤压**：≤360px 时，含表单、购物车、地址、优惠券、发票、SKU 等密集交互块的网格自动折叠为单列（`.has-dense-child` 媒体查询），避免半屏宽的输入框和交易行错位；卡片类网格仍保持多列。
4. **实验室选集数据字段错误**：`episode_list` 契约字段是 `total_episodes|total`，旧 fixture 传了未定义的 `episodes`，组件回退默认 80 集。fixture 已改为 `total_episodes: 2`。
5. **超长文本与长列表**：`text.max_lines` 在 320/390 均正确截断；状态页签新增 80 行 `list`，渲染与滚动正常。客户端对 `repeat` 求值后的动态数组统一截断到 200 项，防止 `$entity` 超大数组绕过发布门禁的 `repeat.items` 上限。
6. **重复提交防护**：同一积木的 `request_data` / `request_payment` 在途时拒绝再次触发；Node 测试覆盖并发两次只发出一次请求、完成后再触发可正常执行。
7. **安全区**：iPhone 12/13 Pro 模拟器读取到底部面板 `padding-bottom: 49px`（30rpx=15px + `env(safe-area-inset-bottom)`=34px），`max-height: 82vh` 生效；真机仍需复验。
8. **评价时间可读性**：服务端返回 RFC3339，评价块直接展示为 `2026-10-30T07:27:56Z`；前端新增本地时间格式化，截图显示 `2026-10-30 15:27`。
9. **实时数据链路验证**：本地 PostgreSQL 冒烟库预置 2 条评价与 4 个商品，`review_summary(auto_load)` 显示 4.5 分/2 条评价，`cart_summary(server_quote)` 显示服务端重算的 ¥39.70；本轮未依赖开发 MySQL。
10. **购物车行价格同步**：`cart_summary` 的服务端报价会写入页面状态 `cart_quote`，`cart_item_list` 优先使用其中的标题与价格；截图证明客户端伪造的“客户端标题/¥0.01”被服务端价格覆盖。
11. **浅色主题对比度**：浅色页发现正文/表单标题沿用固定浅色文字、骨架屏与空态仍是暗色透明底；已把文本块、表单标题、空态标题/描述、骨架屏改为主题变量，并为浅色页补充加载/异常/空态样式。
12. **大字号适配**：读取微信 `fontSizeSetting`（默认 16），按 1.0-1.35 倍写入 `--sdui-font-scale`；SDUI 与导航的 211 处 `font-size` 改为 `calc(... * var(...))`，按钮/输入框/标签改为 `min-height` + 自动高度，RootPortal 弹层单独注入缩放变量。字号 21 时按钮约 18.4px、正文约 21px，表单/购物车/弹层/筛选面板截图无裁切。
13. **登录态优惠券验证**：本地 `mini_apps.app_secret` 为空，无法走微信 code 换 session；本轮用本地 PG 用户/会话生成短期 JWT，并通过 `wx.setStorageSync('hot_mini_user_session', ...)` 注入开发者工具，完成 `coupon.list` 登录态截图。注入的会话与临时生成程序已在验证后删除，不进入仓库。支付仍需商户参数，未做支付截图。

## 分享落地与 WebView 证据

| 文件 | 覆盖内容 | 结果 |
| --- | --- | --- |
| `share-guard-article.png` | AI 付费文章分享链接被未登录用户打开 | 服务端清空积木，客户端显示「该分享需要登录查看」与授权按钮 |
| `webview-share-guard.png` | 带 `auth=1` 的 WebView 分享被未登录用户打开 | 显示「该网页分享需要登录」提示，不加载一次性票据 |
| `home-unified.png` | 首页复用统一 SDUI 容器 | 首页协议正常渲染，分享/字号/错误边界与动态页同源 |
| `state-coupon-live.png` 等 | WebView 字体参数 | 工具读取 `web-view src` 已包含 `sdui_client=weapp&sdui_font_size=16&sdui_font_scale=1.00&sdui_theme=...` |

## 表单间距补充修复

多字段表单最后一个字段与提交按钮之间原先没有额外间距（`form-submit-btn` 仅依赖字段自身外边距），备注输入框会与「提交并加载」贴在一起。已增加 `.sdui-form-multi .form-submit-btn { margin-top: 28rpx; }`，修复后效果见 `form-spacing-fix.png`。

## 本轮新增实时数据证据

| 文件 | 覆盖内容 | 状态 |
| --- | --- | --- |
| `state-review-live.png` | 公开 `review.list` 自动加载：均分、分布、两条真实评价 | 实时数据 |
| `state-cart-quote.png` | `cart_summary` 开启 `server_quote`，按服务端商品配置显示 ¥39.70 | 实时数据 |
| `state-coupon-live.png` | 登录态 `coupon.list` auto_load：¥10 满减券、无门槛、有效期 | 登录态数据 |
| `state-cart-quote-items.png` | 购物车行项目同步服务端报价：协议传“客户端标题/¥0.01”，实际显示“普通会员 ¥9.90”“单篇解锁 ¥19.90” | 实时数据 |

## 覆盖范围

四组证据均覆盖：多字段表单 v2、四类弹层展开态、SKU/数量/购物车/合计、地址簿/地址表单、优惠券/发票、筛选栏/筛选面板、评分/评价、资料卡/用户头部，以及状态页签的空态、骨架屏、缺货、表单 loading/error、真实 `request_data` 异步失败兜底、超长文本截断（`max_lines: 3`）与 80 行长列表。

## 尚未覆盖

- 弹层 safe-area 的真机（非模拟器）复验（模拟器计算样式已验证）；
- 表单输入时的键盘遮挡真机表现：表单与地址/发票/售后输入已补 `cursorSpacing={24}` 与 `adjustPosition`，模拟器无法弹出软键盘，仍需真机复验；
- 超长文本、超长列表的首屏性能与滚动稳定性；
- 多 SKU 结算后端、优惠券/评价持久化。