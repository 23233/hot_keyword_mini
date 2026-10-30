<!-- README.md -->
# SDUI 真机视觉证据（2026-10-29）

环境：微信开发者工具 + `wechatide-skill` v0.3.9，AppID `wx516563cfe994bbc6`，页面 `component_lab_acceptance_20260907`，iPhone 12/13 Pro 模拟器（363×785 原始 PNG）。

生成命令：

```bash
pnpm --dir minifront run build:weapp
node minifront/scripts/capture-sdui-visual.mjs
```

脚本会先回顶测量每个 Block 的绝对位置，再滚动截图；截图的 SHA-256 与尺寸记录在 `manifest.json`。

> 开发者工具的一个模拟器差异：页面滚动后，CSS `position: fixed` 的弹层会按页面坐标定位，面板可能落在视口外。弹层截图统一在打开后回到页面顶部再截；真实设备上的 fixed/safe-area 行为仍需复验。底部动作面板/筛选面板的底部按钮在截图中可能被系统栏裁切，不能据此判定真机安全区通过。

| 文件 | 覆盖内容 | 状态 |
| --- | --- | --- |
| `form-v2.png` | 多字段 `form`：文本、数字、选择、多行文本、开关、提交 | 正常 |
| `dialog-open.png` | 居中确认弹窗、标题、内容、取消/确认 | 展开 |
| `action-sheet-open.png` | 底部动作面板 + 取消 | 展开 |
| `bottom-sheet-open.png` | 底部内容面板 + 确认 | 展开 |
| `sku-selector.png` | SKU 选择、库存、价格、数量、提交；同屏覆盖 `quantity-stepper` | 正常 |
| `quantity-stepper.png` | 数量加减器（与 SKU 同屏） | 正常 |
| `cart-item-list.png` | 购物车商品、数量增减、删除；同屏覆盖 `cart-summary` | 正常 |
| `cart-summary.png` | 购物车合计与结算（与购物车列表同屏） | 正常 |
| `address-book.png` | 地址簿、默认标记、选择/编辑/新增；同屏覆盖 `address-form` | 正常 |
| `address-form.png` | 地址新增表单（与地址簿同屏） | 正常 |
| `coupon-list.png` | 优惠券面额、门槛、有效期、状态；同屏覆盖 `invoice-form` | 正常 |
| `invoice-form.png` | 发票抬头、类型、税号、邮箱（与优惠券同屏） | 正常 |
| `filter-bar.png` | 横向筛选栏与选项态 | 正常 |
| `filter-sheet-open.png` | 底部筛选面板、分组选项、重置/确定 | 展开 |
| `rating.png` | 星级评分 | 正常 |
| `review-summary.png` | 均分、星级分布、评价列表 | 正常 |
| `profile-card.png` | 资料卡、统计项、徽章 | 正常 |
| `user-header.png` | 用户头部、背景、头像、徽章 | 正常 |
| `state-empty.png` | `empty` 空态（状态页签） | 空态 |
| `state-skeleton.png` | `skeleton` 骨架屏（状态页签） | 加载 |
| `state-cart-empty.png` | 购物车空态 | 空态 |
| `state-address-empty.png` | 地址簿空态 | 空态 |
| `state-coupon-empty.png` | 优惠券空态 | 空态 |
| `state-review-empty.png` | 评价空态 | 空态 |
| `state-sku-sold-out.png` | SKU 缺货：规格禁用、数量禁用、提交禁用 | 禁用 |
| `state-form-loading.png` | 表单 `_state: loading` + 块级 loading fallback | 加载 |
| `state-form-error.png` | 表单 `_state: error` + 块级 error fallback | 错误 |
| `state-async-error-open.png` | 真实 `request_data` 失败后自动进入块级 error fallback | 异步错误 |

状态类截图由脚本先点击 `.tab-index-1` 切换到「状态」页签，再逐块滚动截取；异步用例截图前会 `reLaunch` 重置页面，避免上一次点击的块级状态污染证据；清单中 `state` 字段标记为 `variant`。

## 本轮真机截图发现并修复的问题

1. 暗色主题下新组件标题/表单标签使用深色文字，对比度不足；新交互积木改为局部浅色表面 + 局部主题变量。
2. `.sdui-overlay-mask` 在模拟器中高度被撑到整页（约 12993px），弹窗面板被居中到页面中段而不可见；改为 `100vw/100vh` 固定视口。
3. `form.fields` 中带 `type` 的字段对象被 `collectNestedBlocks` 误判为嵌套 BlockItem，导致种子页面校验失败；嵌套块识别改为必须同时具备 `id`。
4. 弹层按钮和触发入口补充稳定 `id`，便于自动化点击与后续视觉回归。
5. 缺货 SKU 的提交按钮原先只降透明度，浅蓝底白字几乎不可读；改为灰底灰字，规格卡与数量控件同步禁用。
6. 骨架屏线条在暗色卡片上对比度过低，透明度由 `0.05/0.12` 提升到 `0.08/0.18`。
7. 弹层打开时底层 `ScrollView` 仍可滚动；页面新增 `setOverlayOpen` 回调，dialog/action_sheet/bottom_sheet/filter_sheet 打开时关闭 `scrollY` 并追加 `is-overlay-locked`。已在模拟器读取到 `scroll-y=false`，关闭后恢复 `true`。
8. 状态页签新增真实异步用例：`request_data` 携带 `target` 时，点击提交先进入块级 loading，成功后恢复、失败后自动切到块级 error fallback；截图 `state-async-error-open.png` 为匿名请求 `logistics.refresh` 失败后的真实兜底，单元测试同时断言 `loading → normal/error` 的状态序列。

## 开发者工具缓存提示

微信开发者工具会按项目缓存 appservice 编译产物。仅执行 `simulator_refresh` 可能继续运行旧的 JS（WXSS 却会更新），表现为“样式变了、逻辑没变”。改动前端 JS 后应使用 `debug_clear_cache` 或删除 `%LOCALAPPDATA%\微信开发者工具\User Data\<账号>\WeappCache\WeappCompileCache\<项目哈希>`，再重开项目窗口。本目录截图在清缓存并确认新代码后生成。

## 尚未覆盖

- 弹层 safe-area、遮罩下滚动手势的真机复验；
- 多字段表单的键盘遮挡与输入态；
- 窄屏（SE）与长文本/长列表；
- 协议驱动的动态 `loading/empty/error`（当前通过 `_state` 与块级 fallback 验证，未接真实异步接口）；
- 优惠券/评价的真实后端数据与持久化。