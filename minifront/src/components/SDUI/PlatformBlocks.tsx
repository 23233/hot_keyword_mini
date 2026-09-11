// PlatformBlocks.tsx
import React from 'react'
import { Button, Image, Map, Progress, Text, View } from '@tarojs/components'
import { BlockAction, BlockItem } from '../../types/sdui'

interface PlatformBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

const trigger = (block: BlockItem, onAction?: PlatformBlockProps['onAction'], item?: Record<string, any>, action?: BlockAction) => {
  const target = action || block.action
  if (target || block.events?.tap) {
    onAction?.(target, { item: item || block.props || {}, actionPayload: item || block.props || {} })
  }
}

/** BottomNavBlock 渲染由 SDUI 配置的固定底部导航。 */
export const BottomNavBlock: React.FC<PlatformBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const items = Array.isArray(props.items) ? props.items.filter((item: any) => item && item.hidden !== true) : []
  const activeKey = String(props.active_key || '')
  return (
    <View className='sdui-bottom-nav-space'>
      <View className='sdui-bottom-nav'>
        {items.map((item: any, index: number) => {
          const key = String(item.key || item.page_id || index)
          const action = item.action || { type: 'navigate_page', payload: { page_id: item.page_id || 'home', query: item.query || {} } }
          return (
            <View key={key} className={`bottom-nav-item ${key === activeKey ? 'is-active' : ''}`} onClick={(event) => {
              event?.stopPropagation?.()
              trigger(block, onAction, item, action)
            }}>
              {item.icon_url && <Image className='bottom-nav-icon' src={String(item.icon_url)} mode='aspectFit' />}
              <Text className='bottom-nav-label'>{String(item.label || item.title || '')}</Text>
            </View>
          )
        })}
      </View>
    </View>
  )
}

/** FloatingActionBlock 渲染可配置的安全区浮动操作入口。 */
export const FloatingActionBlock: React.FC<PlatformBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  return (
    <View className={`sdui-floating-action position-${props.position || 'right'}`} onClick={(event) => {
      event?.stopPropagation?.()
      trigger(block, onAction)
    }}>
      {props.icon_url && <Image className='floating-action-icon' src={String(props.icon_url)} mode='aspectFit' />}
      {props.label && <Text className='floating-action-label'>{String(props.label)}</Text>}
    </View>
  )
}

/** GameHeaderBlock 渲染可复用于任意游戏数据源的详情头部。 */
export const GameHeaderBlock: React.FC<PlatformBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const tags = Array.isArray(props.tags) ? props.tags : []
  return (
    <View className='sdui-game-header' onClick={() => trigger(block, onAction)}>
      {props.cover_url && <Image className='game-header-cover' src={String(props.cover_url)} mode='aspectFill' />}
      <View className='game-header-body'>
        <View className='game-header-identity'>
          {props.icon_url && <Image className='game-header-icon' src={String(props.icon_url)} mode='aspectFill' />}
          <View className='game-header-copy'>
            <Text className='game-header-title'>{String(props.title || props.name || '游戏详情')}</Text>
            {(props.version || props.publisher) && <Text className='game-header-meta'>{[props.version, props.publisher].filter(Boolean).join(' · ')}</Text>}
          </View>
        </View>
        {tags.length > 0 && <View className='game-header-tags'>{tags.map((tag: any, index: number) => <Text key={`${tag}_${index}`} className='game-header-tag'>{String(tag)}</Text>)}</View>}
        {(props.description || props.desc) && <Text className='game-header-desc'>{String(props.description || props.desc)}</Text>}
        {props.btn_text && <Button className='game-header-button' onClick={(event) => {
          event?.stopPropagation?.()
          trigger(block, onAction, props, props.btn_action)
        }}>{String(props.btn_text)}</Button>}
      </View>
    </View>
  )
}

/** MediaPickerBlock 提供统一媒体选择入口，上传由 Action 执行器负责。 */
export const MediaPickerBlock: React.FC<PlatformBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const items = Array.isArray(props.items) ? props.items : []
  const maxCount = Number(props.max_count || 9)
  return (
    <View className='sdui-media-picker'>
      {props.title && <Text className='platform-block-title'>{String(props.title)}</Text>}
      <View className='media-picker-grid'>
        {items.map((item: any, index: number) => <Image key={item.id || item.url || index} className='media-picker-thumb' src={String(item.url || item.path || item)} mode='aspectFill' />)}
        {items.length < maxCount && <Button className='media-picker-add' onClick={(event) => {
          event?.stopPropagation?.()
          trigger(block, onAction, props, block.action || { type: 'choose_media', payload: { count: maxCount - items.length } })
        }}>+</Button>}
      </View>
      {props.hint && <Text className='platform-block-hint'>{String(props.hint)}</Text>}
    </View>
  )
}

/** UploadProgressBlock 展示稳定尺寸的上传进度和状态。 */
export const UploadProgressBlock: React.FC<PlatformBlockProps> = ({ block }) => {
  const props = block.props || {}
  const progress = Math.max(0, Math.min(100, Number(props.progress || 0)))
  return <View className='sdui-upload-progress'>
    <View className='upload-progress-row'><Text>{String(props.title || '正在上传')}</Text><Text>{progress}%</Text></View>
    <Progress percent={progress} strokeWidth={4} activeColor={String(props.accent_color || '#0a84ff')} backgroundColor='#d1d1d6' />
    {props.status_text && <Text className='platform-block-hint'>{String(props.status_text)}</Text>}
  </View>
}

/** MediaGalleryBlock 展示通用图片/视频缩略图集合。 */
export const MediaGalleryBlock: React.FC<PlatformBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const items = Array.isArray(props.items) ? props.items : []
  return <View className='sdui-media-gallery'>
    {props.title && <Text className='platform-block-title'>{String(props.title)}</Text>}
    <View className='media-gallery-grid'>{items.map((item: any, index: number) => {
      const url = String(item.url || item.image_url || item)
      return <Image key={item.id || url || index} className='media-gallery-item' src={url} mode='aspectFill' onClick={(event) => {
        event?.stopPropagation?.()
        trigger(block, onAction, item, item.action || { type: 'preview_image', payload: { current: url, urls: items.map((entry: any) => entry.url || entry.image_url || entry) } })
      }} />
    })}</View>
  </View>
}

/** TencentMapBlock 使用通用坐标协议渲染腾讯地图小程序组件。 */
export const TencentMapBlock: React.FC<PlatformBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const latitude = Number(props.latitude)
  const longitude = Number(props.longitude)
  const valid = Number.isFinite(latitude) && latitude >= -90 && latitude <= 90 && Number.isFinite(longitude) && longitude >= -180 && longitude <= 180
  const isPicker = block.type === 'location_picker'
  return <View className='sdui-map-block'>
    <View className='platform-block-header'>
      <View><Text className='platform-block-title'>{String(props.title || '地图位置')}</Text>{props.address && <Text className='platform-block-hint'>{String(props.address)}</Text>}</View>
      <Button className='platform-inline-button' onClick={(event) => {
        event?.stopPropagation?.()
        trigger(block, onAction, props, block.action || (isPicker ? { type: 'choose_location' } : { type: 'open_map', payload: props }))
      }}>{String(props.btn_text || (isPicker ? '选择位置' : '导航'))}</Button>
    </View>
    {valid ? <Map className='tencent-map-view' latitude={latitude} longitude={longitude} scale={Number(props.scale || 16)} markers={[{ id: 1, latitude, longitude, title: String(props.title || props.address || '') }] as any} onError={() => {}} /> : <View className='platform-empty'><Text>暂未配置有效位置</Text></View>}
  </View>
}

/** ContactServiceBlock 默认使用微信内置客服组件，可切换平台内客服。 */
export const ContactServiceBlock: React.FC<PlatformBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const mode = String(props.mode || 'wechat')
  const content = <><Text className='platform-block-title'>{String(props.title || '联系客服')}</Text>{props.description && <Text className='platform-block-hint'>{String(props.description)}</Text>}</>
  if (mode === 'wechat') {
    return <View className='sdui-contact-service'>{content}<Button className='platform-primary-button' openType='contact' sessionFrom={String(props.session_from || 'sdui')}>{String(props.btn_text || '联系微信客服')}</Button></View>
  }
  return <View className='sdui-contact-service'>{content}<Button className='platform-primary-button' onClick={(event) => {
    event?.stopPropagation?.()
    trigger(block, onAction, props, block.action || { type: 'open_internal_chat', payload: { page_id: props.page_id || 'customer_service', context: props.context || {} } })
  }}>{String(props.btn_text || '进入在线客服')}</Button></View>
}

/** WebViewEntryBlock 展示受控 WebView 入口及不可用状态。 */
export const WebViewEntryBlock: React.FC<PlatformBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const disabled = props.disabled === true || props.status === 'disabled' || props.status === 'blocked'
  return <View className={`sdui-webview-entry ${disabled ? 'is-disabled' : ''}`}>
    <View><Text className='platform-block-title'>{String(props.title || '打开网页')}</Text>{props.description && <Text className='platform-block-hint'>{String(props.description)}</Text>}</View>
    <Button className='platform-inline-button' disabled={disabled} onClick={(event) => {
      event?.stopPropagation?.()
      trigger(block, onAction, props)
    }}>{String(props.btn_text || '打开')}</Button>
  </View>
}
