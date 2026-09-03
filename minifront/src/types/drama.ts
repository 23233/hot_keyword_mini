// 播放模式类型 (通用底层播放规则)
export type PlayMode = 'direct_video' | 'channels_embedded' | 'channels_video' | 'web_view' | 'none'

// 展示模式类型 (后端接口完全驱动)
export type DisplayMode = 'immersive_video' | 'episode_grid' | 'direct_portal' | 'gallery_matrix' | 'webview'

// 首页悬浮按钮配置(由后端动态下发，有则展示)
export interface FloatingButton {
  text: string
  icon?: string
  action_type: 'open_modal' | 'copy_pan' | 'jump_channels'
  action_data?: string
  badge?: string
  is_visible: boolean
}

// 短剧基础信息类型 (由后端接口完全驱动)
export interface DramaInfo {
  id: number
  title: string
  subtitle: string
  cover_url: string
  banner_url: string
  total_episodes: number
  updated_episodes: number
  rating: number
  hot_score: number
  tags: string
  description: string
  highlights: string
  play_mode?: PlayMode
  finder_user_name?: string
  channels_feed_id?: string
  web_url?: string
}

// 剧集选集信息类型
export interface EpisodeItem {
  id: number
  drama_id: number
  episode_num: number
  title: string
  cover_url: string
  video_url: string
  is_free: boolean
  duration: number
  play_mode?: PlayMode
  finder_user_name?: string
  channels_feed_id?: string
  web_url?: string
}

// 看后续转化渠道配置项类型 (如有网盘数据就显示网盘)
export interface ActionChannel {
  type: 'pan' | 'mp' | 'customer' | 'mini'
  name: string
  icon: string
  desc: string
  btn_text: string
  content: string
  fetch_code?: string
  target_appid?: string
  target_path?: string
  tip_notice: string
}

// 首页接口返回的完整数据类型 (零写死，全部依赖接口)
export interface DramaHomeData {
  page_title: string
  page_subtitle: string
  drama: DramaInfo
  episodes: EpisodeItem[]
  display_mode: DisplayMode
  webview_url?: string
  action_channels: ActionChannel[]
  floating_button?: FloatingButton
  announcement?: string
  share_title: string
  share_desc: string
  share_cover: string
  recommendations: DramaInfo[]
  gallery_list: DramaInfo[]
}

// 统一 API 响应格式
export interface ApiResponse<T> {
  code: number
  msg: string
  data: T
}
