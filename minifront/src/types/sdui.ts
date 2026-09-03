/**
 * SDUI 服务端驱动动态组件协议类型定义
 * 严格与 Golang 后端 models/sdui.go 双端对齐
 */

// 标准原子动作类型枚举
export type BlockActionType =
  | 'copy_text'              // 复制文本至剪贴板
  | 'navigate_page'          // 小程序内部页面路由跳转
  | 'open_channels_activity' // 直达微信视频号原生动态
  | 'open_mini_program'      // 跨小程序矩阵跳转
  | 'preview_image'          // 全屏预览大图
  | 'open_webview'           // 网页 H5 容器打开
  | 'request_data'           // 业务接口数据请求
  | 'require_auth'           // 强制登录拦截
  | 'toast'                  // 纯文字气泡提示
  | 'refresh'                // 刷新当前页面或指定块
  | 'share'                  // 唤起微信分享面板

// 原子动作协议结构
export interface BlockAction {
  // 动作类型
  type: BlockActionType
  // 是否必须要求已登录态才可执行
  require_auth?: boolean
  // 动作参数载荷字典
  payload?: Record<string, any>
}

// 积木组件视觉渲染样式 (严格遵循苹果 HIG 设计标记)
export interface BlockStyle {
  // 垂直外边距
  margin_y?: string
  // 水平外边距
  margin_x?: string
  // 内边距
  padding?: string
  // 圆角梯度 (如 24rpx, 999rpx)
  border_radius?: string
  // 是否启用苹果毛玻璃磨砂特效 (backdrop-filter: blur)
  glass_blur?: boolean
  // 主题高光强调色
  accent_color?: string
  // 背景色/渐变
  background?: string
}

// 原子积木组件核心数据结构
export interface BlockItem {
  // 积木唯一标识
  id: string
  // 积木类型 (如 media_hero, resource_card, action_button, notice, item_grid 等)
  type: string
  // 组件业务属性
  props?: Record<string, any>
  // 视觉样式
  style?: BlockStyle
  // 单一交互动作
  action?: BlockAction
  // 多事件流绑定 (如 tap: [Action1, Action2])
  events?: Record<string, BlockAction[]>
  // 受控显示条件
  visible_when?: Record<string, any>
  // 列表循环配置
  repeat?: Record<string, any>
  // 兜底降级组件
  fallback?: BlockItem
}

// 微信分享配置项
export interface ShareItem {
  // 是否启用该渠道
  enabled: boolean
  // 分享标题
  title?: string
  // 分享好友目标路径
  path?: string
  // 分享朋友圈查询参数
  query?: string
  // 分享卡片海报图
  image_url?: string
}

// 页面级微信分享整体配置
export interface PageShareConfig {
  // 默认分享兜底图
  default_image_url?: string
  // 分享给微信好友配置
  friend?: ShareItem
  // 分享到朋友圈配置
  timeline?: ShareItem
}

// 动态页面实体传输对象
export interface DynamicPageDTO {
  // 页面唯一标识 (如 home / drama_detail)
  page_id: string
  // 修订版本号
  revision: number
  // 页面发布状态 (published / draft)
  status: string
  // 页面主标题
  title: string
  // 业务类型 (drama / game / query / download / custom)
  business_type: string
  // 用户搜索意图 (watch / redeem / query / download)
  intent: string
  // 主题基调 (dark_glass / light_clean / cyber_neon)
  theme: string
  // 主题强调色
  accent_color: string
  // 是否全页强制登录
  require_auth: boolean
  // 微信分享配置
  share_config?: PageShareConfig
  // 原子积木列表
  blocks: BlockItem[]
}

// 缓存控制元数据
export interface EnvelopeCache {
  // ETag 实体标识
  etag: string
  // 客户端缓存最大有效期(秒)
  max_age: number
}

// 降级兜底方案
export interface EnvelopeFallback {
  // 兜底页面标识
  page_id: string
  // 兜底模式
  mode: string
}

// 服务端驱动统一响应信封
export interface PageResponseEnvelope {
  // 协议版本 (如 "1.1")
  protocol_version: string
  // Schema 结构版本 (如 3)
  schema_version: number
  // 请求追踪 ID
  request_id: string
  // 动态页面主体
  page: DynamicPageDTO
  // 受控附加实体数据
  data: Record<string, any>
  // 渲染所需的客户端核心能力
  capabilities_required: string[]
  // 缓存控制
  cache: EnvelopeCache
  // 异常降级
  fallback: EnvelopeFallback
}

// 本地登录会话状态实体
export interface UserSessionState {
  access_token: string
  access_expires_at: string
  refresh_token: string
  refresh_expires_at: string
  session_id: string
  user: {
    id: number
    app_id?: string
    nick_name: string
    avatar_url?: string
    wechat_open_id?: string
  }
}
